package main

/* @todo
WEBVIEW
- To try cache ui files
- To track update files
- To swift to tailwind

CSV
- To get sample
	- To show it in the interface
- To change encoding
- To make replace values in the interface
- To try , then ; to identify separator
- To make conditions
*/
import (
	"csv-slicer/cmd/utils"
	"fmt"
	"io/fs"
	"math"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"

	"embed"
	"encoding/csv"
	"encoding/json"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ncruces/zenity"
	webview "github.com/webview/webview_go"
)

type Tick struct {
	fname string
	tick  time.Duration
}

type FilterCond struct {
	Cond string `hcl:"cond"`
	Val  string `hcl:"val"`
}

// idMotivo
// pmf.agenda_incidente_tarefa_atu

//go:embed all:ui/*
var assets embed.FS

var (
	screenW = 900
	screenH = 900
	w       webview.WebView

	err         error
	sourceFiles = [][2]string{}

	totFiles          int
	totLines          int
	totLinesPerSource int
	lines             []string
	header            string
	filterColumns     []string
	filter_cols       bool
	sep_target        string
	is_sql            bool

	cols_cond   map[int][]FilterCond
	cols_format map[int][]int

	format_ONLY_DIGITS = 1

	targetSqlTable  = ""
	targetDirBase   = ""
	targetMaxLines  = 0
	targetLineSlice = 100000
	targetFiles     = []string{}
	targetRenames   map[string]string
	targetToDigit   map[string]bool
	targetReplaces  map[int]map[string]string

	sourceDirBase   = "target"
	sourceSeparator = ","

	times     = []Tick{}
	last_tick = time.Now()
)

func init() {
	runtime.LockOSThread()
}

func main() {
	url, err := startServer()
	if err != nil {
		panic(err)
	}

	w = webview.New(true)
	defer w.Destroy()

	setupWindow(url + "/pages/home.html")
	registerWebviewMethods()
	w.Run()
}

func startServer() (string, error) {
	sub, err := fs.Sub(assets, "ui")
	if err != nil {
		return "", err
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))

	// Listen on a random free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	go func() {
		if err := http.Serve(ln, mux); err != nil && err != http.ErrServerClosed {
			fmt.Println(err)
			os.Exit(1)
		}
	}()

	return "http://" + ln.Addr().String(), nil
}

func setupWindow(url string) {
	w.SetTitle("CSV Slicer")
	w.SetSize(screenW, screenH, webview.HintFixed)
	// w.SetHtml()
	fmt.Println(url)
	w.Navigate(url)
}

func registerWebviewMethods() {
	registerMethod("openTestCSV", openTestCSV)
	registerMethod("openCSV", openCSV)
	registerMethod("procCSV", procCSV)
}

func registerMethod(method string, fn_addr any) {
	if err := w.Bind(method, fn_addr); err != nil {
		panic(err)
	}
}

func openTestCSV() [][2]string {
	sourceFiles = [][2]string{}
	dir, _ := os.Getwd()
	fname := dir + "/source/test.csv"
	info, err := os.Stat(fname)
	if err != nil {
		w.Eval(`appSetHeader([])`)
		return sourceFiles
	}

	realfname, err := filepath.EvalSymlinks(fname)
	if err != nil {
		panic(err)
	}
	size_txt := utils.FormatSize(info.Size())
	sourceFiles = append(sourceFiles, [2]string{realfname, size_txt})

	cur_header := getOriginalHeader(fname)
	data, _ := json.Marshal(cur_header)
	json_header := string(data)
	w.Eval("appSetHeader(" + json_header + ")")

	return sourceFiles
}

func openCSV() [][2]string {
	sourceFiles = [][2]string{}
	dir, _ := os.Getwd()
	files, err := zenity.SelectFileMultiple(
		zenity.Title("Escolha o arquivo para comprimir"),
		zenity.Modal(),
		zenity.Filename(dir),
		zenity.FileFilters{
			{
				Name:     "CSV Files",
				Patterns: []string{"*.csv"},
			},
		},
	)

	var original_header []string
	headers_equal := true

	if err != nil {
		w.Eval(`appSetHeader([])`)
		return sourceFiles
	}

	for i, fname := range files {
		info, _ := os.Stat(fname)
		size_txt := utils.FormatSize(info.Size())
		sourceFiles = append(sourceFiles, [2]string{fname, size_txt})

		cur_header := getOriginalHeader(fname)
		if i == 0 {
			original_header = cur_header
			continue
		} else if headers_equal && !slices.Equal(cur_header, original_header) {
			headers_equal = false
		}
	}

	w.Dispatch(func() {
		if headers_equal {
			data, _ := json.Marshal(original_header)
			json_header := string(data)
			w.Eval("appSetHeader(" + json_header + ")")
		} else {
			w.Eval(`appSetHeader([])`)
		}
	})

	return sourceFiles
}

func procCSV(sourceSep string,
	maxLines int,
	lineSlice int,
	sqlTable string,
	cols []string,
	renames map[string]string,
	toDigit map[string]bool,
	replaces map[string]map[string]string) {
	filterColumns = cols
	sourceSeparator = sourceSep

	targetSqlTable = sqlTable
	targetToDigit = toDigit
	targetRenames = renames
	targetMaxLines = maxLines
	targetLineSlice = lineSlice

	totLinesPerSource = 0
	totFiles = 0
	targetFiles = []string{}
	times = []Tick{}

	prepareTest()
	prepareImportation()

	go func() {
		for _, item := range sourceFiles {
			totLines = 0
			fmt.Println("PROCESSING FILE:", item[0])
			fsource_name := item[0]
			file := utils.OpenFileOrError(fsource_name, "Problem to open the file ["+fsource_name+"].")
			defer file.Close()

			// START READING
			reader := utils.NewCsvReader(file, []rune(sourceSeparator)[0])
			getHeader(reader, replaces)

			// READ LINES
			for {
				record, must_break := utils.ReadCsvLine(reader)
				if must_break {
					break
				}

				if !getCsvLine(record) {
					continue
				}

				totLines++
				totLinesPerSource++

				if totLines == targetLineSlice {
					saveSlice(item[0])
				}

				if targetMaxLines != 0 && totLinesPerSource == targetMaxLines {
					saveSlice(item[0])
					break
				}
			}

			if totLines > 0 {
				saveSlice(item[0])
			}
		}

		w.Dispatch(func() {
			w.Eval("appFinishProcess()")
		})

		log()
	}() // PREPARE SOURCE
}

func prepareTest() {
	// dir, _ := os.Getwd()
	// targetDirBase = dir + "target/"
	targetDirBase = "target/"
	sep_target = ","
	/*
		if !cfg.Test.Activated {
			return
		}

		cfg.Source.Files = cfg.Test.Files
		cfg.Source.DirBase = cfg.Test.DirBase
		cfg.Target.MaxLinesPerFile = cfg.Test.MaxLinesPerFile
	*/
}

func prepareImportation() {
	os.RemoveAll("target")
	os.Mkdir("target", 0644)

	is_sql = targetSqlTable != ""
}

func getOriginalHeader(fname string) []string {
	file := utils.OpenFileOrError(fname, "Problem to open the file ["+fname+"].")
	defer file.Close()
	reader := utils.NewCsvReader(file, []rune(sourceSeparator)[0])
	record, _ := utils.ReadCsvLine(reader)

	return record
}

func getHeader(reader *csv.Reader, replaces map[string]map[string]string) {
	tmp_cols := map[string]int{}
	cols_cond = map[int][]FilterCond{}
	cols_format = map[int][]int{}
	record, _ := utils.ReadCsvLine(reader)
	targetReplaces = map[int]map[string]string{}
	cols := []string{}
	encloser := "\""

	if is_sql {
		encloser = "`"
		sep_target = ", "
	}

	for i, val := range record {
		jump := true
		for _, field := range filterColumns {
			if field == val {
				jump = false
				break
			}
		}
		if jump {
			continue
		}

		tmp_cols[val] = i
		cols_cond[i] = []FilterCond{}
		cols_format[i] = []int{}
		if _, ok := targetToDigit[val]; ok {
			cols_format[i] = append(cols_format[i], format_ONLY_DIGITS)
		}
		if replace, ok := replaces[val]; ok {
			targetReplaces[i] = replace
		}
		if new_name, ok := targetRenames[val]; ok {
			val = new_name
		}
		cols = append(cols, encloser+val+encloser)
	}

	/*
		for field, filts := range cfg.Filter.Conds {
			if idx, ok := tmp_cols[field]; ok {
				cols_cond[idx] = filts
			}
		}
	*/

	header = strings.Join(cols, sep_target)
	if is_sql {
		header = "INSERT IGNORE INTO " + targetSqlTable + "(" + header + ") VALUES"
	}

	lines = []string{header}
}

func getCsvLine(record []string) bool {
	vals := []string{}
	for i, val := range record {
		if _, ok := cols_format[i]; !ok {
			continue
		}

		for _, format_cod := range cols_format[i] {
			if format_cod == format_ONLY_DIGITS {
				// @todo
				re := regexp.MustCompile(`(\D+)`)
				val = re.ReplaceAllStringFunc(val, func(match string) string {
					return ""
				})
				break
			}
		}

		if filter_cols {
			if filts, ok := cols_cond[i]; !ok {
				continue
			} else {
				for _, filt := range filts {
					switch filt.Cond {
					case "=":
						if val != filt.Val {
							return false
						}
					case "!=":
						if val == filt.Val {
							return false
						}
					case ">":
						if val <= filt.Val {
							return false
						}
					case ">=":
						if val < filt.Val {
							return false
						}
					case "<":
						if val >= filt.Val {
							return false
						}
					case "<=":
						if val > filt.Val {
							return false
						}
					}
				}
			}
		}

		if replace, ok := targetReplaces[i][val]; ok {
			val = replace
		}
		if val[0] == '0' && len(val) > 1 {
			vals = append(vals, "\""+val+"\"")
		} else if _, err := strconv.Atoi(val); err != nil {
			vals = append(vals, "\""+val+"\"")
		} else {
			vals = append(vals, val)
		}
	}

	line := strings.Join(vals, sep_target)
	if is_sql {
		line = "(" + line + "),"
	}

	lines = append(lines, line)

	return true
}

func saveSlice(fname string) {
	totFiles++
	files_txt := strconv.Itoa(totFiles)
	pos := strings.LastIndex(fname, "\\")
	ftarget := fname[pos+1 : len(fname)-4]
	ftarget += "--" + files_txt + "--"
	reg_ini := (totFiles-1)*targetLineSlice + 1
	reg_fim := reg_ini + totLines - 1
	ftarget += strconv.Itoa(reg_ini) + "-" + strconv.Itoa(reg_fim) + ".csv"
	if is_sql {
		lines[totLines] = lines[totLines][:len(lines[totLines])-1]
	}
	content := strings.Join(lines, "\n")
	os.WriteFile(targetDirBase+ftarget, []byte(content), 0644)
	times = append(times, Tick{ftarget, time.Since(last_tick)})
	last_tick = time.Now()
	targetFiles = append(targetFiles, targetDirBase+ftarget)
	w.Dispatch(func() {
		w.Eval("appAddFileTarget(\"" + targetDirBase + ftarget + "\")")
	})

	totLines = 0
	totLinesPerSource = 0
	lines = []string{header}
}

func log() {
	fmt.Println("\nDUTATION:")
	fmt.Println("---------")
	var total float64
	for i, time := range times {
		secs := math.Round(time.tick.Seconds()*100) / 100
		total += secs
		fmt.Println(i+1, time.fname, "-", secs, "seconds")
	}
	total = math.Round(total*100) / 100
	fmt.Println("---------------------")
	fmt.Println("TOTAL ", total, "seconds")
}
