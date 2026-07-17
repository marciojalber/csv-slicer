// src/main.go

package main

import (
	"encoding/csv"
	"fmt"
	"importacoes/code/utils"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// Removidos os registros com protocAgenda = 0
// Removidos os registros com agendamento duplicado para tarefas diferentes
// Removidos os agendamentos com agendamento duplicado para tarefas diferentes
//  32 casos - Removidos os agendamentos com hora > 23:59:59
// 546 casos - Alguns agendamentos não foram encontrados

// - 1383865 - 1739360 - 1707401 - 1203769

type Tick struct {
	fname string
	tick  time.Duration
}

type Config struct {
	Test   TestConfig   `hcl:"test,block"`
	Source SourceConfig `hcl:"source,block"`
	Target TargetConfig `hcl:"target,block"`
	Filter FilterConfig `hcl:"filter,block"`
}

type TestConfig struct {
	Activated       bool     `hcl:"activated"`
	DirBase         string   `hcl:"dirBase"`
	Files           []string `hcl:"files"`
	MaxLinesPerFile int      `hcl:"maxLinesPerFile"`
}

type SourceConfig struct {
	Files     []string `hcl:"files"`
	DirBase   string   `hcl:"dirBase"`
	Separator string   `hcl:"separator"`
}

type TargetConfig struct {
	DirBase         string `hcl:"dirBase"`
	SqlTable        string `hcl:"_sqlTable,optional"`
	MaxLinesPerFile int    `hcl:"maxLinesPerFile"`
	Separator       string `hcl:"_separator,optional"`
}

type FilterConfig struct {
	Columns []string      `hcl:"_columns,optional"`
	Fields  []FilterField `hcl:"_field,block"`
	Conds   map[string][]FilterCond
}

type FilterField struct {
	Name   string       `hcl:"name,label"`
	Filter []FilterCond `hcl:"filter,block"`
}

type FilterCond struct {
	Cond string `hcl:"cond"`
	Val  string `hcl:"val"`
}

var (
	cfg Config

	fields = map[string]func(val string) bool{
		"protocAgenda": func(val string) bool {
			return val != "0"
		},
	}

	// RECORDS
	filter_cols bool
	is_sql      bool
	i_cols      map[int][]FilterCond
	sep_target  string
	lines       []string
	files       int
	total       = 0
	header      string

	// TICK
	times     = []Tick{}
	last_tick = time.Now()
)

func main() {
	initConfig()
	prepareTest()
	prepareImportation()

	// PREPARE SOURCE
	for _, fname := range cfg.Source.Files {
		files = 0
		fmt.Println("PROCESSING FILE:", fname)
		fsource_name := cfg.Source.DirBase + "/" + fname
		file := utils.OpenFileOrError(fsource_name, "Problem to open the file ["+fsource_name+"].")
		defer file.Close()

		// START READING
		reader := utils.NewCsvReader(file, []rune(cfg.Source.Separator)[0])
		getHeader(reader)

		// READ LINES
		for {
			record, must_break := utils.ReadCsvLine(reader)
			if must_break {
				break
			}

			if !getCsvLine(record) {
				continue
			}

			total++

			if total == cfg.Target.MaxLinesPerFile {
				saveSlice(fname)
			}
		}

		if total > 0 {
			saveSlice(fname)
		}
	}

	log()
}

func initConfig() {
	parser := hclparse.NewParser()

	file, diags := parser.ParseHCLFile("config.hcl")
	if diags.HasErrors() {
		panic(diags.Error())
	}

	diags = gohcl.DecodeBody(file.Body, nil, &cfg)
	if diags.HasErrors() {
		panic(diags.Error())
	}

	if cfg.Target.Separator == "" {
		sep_target = cfg.Source.Separator
	} else {
		sep_target = cfg.Target.Separator
	}

	filter_cols = len(cfg.Filter.Columns) != 0
	cfg.Filter.Conds = map[string][]FilterCond{}

	for _, filt := range cfg.Filter.Fields {
		cfg.Filter.Conds[filt.Name] = filt.Filter
	}
}

func prepareTest() {
	if !cfg.Test.Activated {
		return
	}

	cfg.Source.Files = cfg.Test.Files
	cfg.Source.DirBase = cfg.Test.DirBase
	cfg.Target.MaxLinesPerFile = cfg.Test.MaxLinesPerFile
}

func prepareImportation() {
	os.RemoveAll(cfg.Target.DirBase)
	os.Mkdir(cfg.Target.DirBase, 0644)

	is_sql = cfg.Target.SqlTable != ""
}

func getHeader(reader *csv.Reader) {
	tmp_cols := map[string]int{}
	i_cols = map[int][]FilterCond{}
	record, _ := utils.ReadCsvLine(reader)
	cols := []string{}
	encloser := "\""

	if is_sql {
		encloser = "`"
		sep_target = ", "
	}

	for i, val := range record {
		if filter_cols {
			jump := true
			for _, field := range cfg.Filter.Columns {
				if field == val {
					jump = false
					break
				}
			}
			if jump {
				continue
			}
		}

		tmp_cols[val] = i
		i_cols[i] = []FilterCond{}
		cols = append(cols, encloser+val+encloser)
	}

	for field, filts := range cfg.Filter.Conds {
		if idx, ok := tmp_cols[field]; ok {
			i_cols[idx] = filts
		}
	}

	header = strings.Join(cols, sep_target)
	if is_sql {
		header = "INSERT IGNORE INTO " + cfg.Target.SqlTable + "(" + header + ") VALUES"
	}

	lines = []string{header}
}

func getCsvLine(record []string) bool {
	vals := []string{}
	for i, val := range record {
		if filter_cols {
			if filts, ok := i_cols[i]; !ok {
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
		if val[0] == '0' && len(val) > 1 {
			vals = append(vals, "\""+record[i]+"\"")
		} else if _, err := strconv.Atoi(val); err != nil {
			vals = append(vals, "\""+record[i]+"\"")
		} else {
			vals = append(vals, record[i])
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
	files++
	files_txt := strconv.Itoa(files)
	ftarget := fname[:len(fname)-4]
	ftarget += "--" + files_txt + "--"
	reg_ini := (files-1)*cfg.Target.MaxLinesPerFile + 1
	reg_fim := reg_ini + total - 1
	ftarget += strconv.Itoa(reg_ini) + "-" + strconv.Itoa(reg_fim) + ".csv"
	if is_sql {
		lines[total] = lines[total][:len(lines[total])-1]
	}
	content := strings.Join(lines, "\n")
	os.WriteFile(cfg.Target.DirBase+"/"+ftarget, []byte(content), 0644)
	times = append(times, Tick{ftarget, time.Since(last_tick)})
	last_tick = time.Now()

	total = 0
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
