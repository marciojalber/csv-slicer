package main

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type UI struct {
	res *embed.FS
}

var (
	ui         *UI
	basepath   = "templates/pages/"
	tracking   map[string]int64
	track_name = "__tracking.json"
)

// Create new files in the new path
// Link old ones

func main() {
	old_path := "ui/pages_old_" + strconv.FormatInt(time.Now().Unix(), 36)
	fmt.Println(old_path)
	os.RemoveAll("ui/_pages")
	os.Mkdir("ui/_pages", 0644)
	parseDir("")
	os.Rename("ui/pages", old_path)
	os.Rename("ui/_pages", "ui/pages")
	os.RemoveAll(old_path)
}

func parseDir(path string) {
	fpath := basepath + path
	use_path := path
	if use_path != "" {
		use_path += "/"
	}
	dir, _ := os.ReadDir(fpath)
	for _, item := range dir {
		if item.IsDir() {
			parseDir(use_path + item.Name())
			continue
		}

		if item.Name() != "__index.html" {
			continue
		}

		fmt.Println("\n" + fpath + ":")
		fmt.Println(strings.Repeat("-", len(fpath)))

		if !mustRebuild(fpath) {
			fmt.Println("  File is updated.")
			continue
		}

		MakePage(path)
	}
}

func MakePage(fname string) {
	tracking = map[string]int64{}
	fpath := "templates/pages/" + fname
	pos := strings.LastIndex(fname, "/")
	dir := "ui/_pages"
	if pos != -1 {
		dir += "/" + fname[:pos]
	}
	pname := fname[pos+1:] + ".html"
	if err := os.MkdirAll(dir, 0644); err != nil {
		finishWithError(err)
	}

	depends := []string{}
	html := parse(fpath+"/__index.html", "", depends)
	os.WriteFile(dir+"/"+pname, []byte(html), 0644)

	data, _ := json.MarshalIndent(tracking, "", "    ")
	os.WriteFile(fpath+"/"+track_name, []byte(data), 0644)
}

func parse(fname, content string, depends []string) string {
	pos := strings.LastIndex(fname, "/")
	fbase := fname[:pos]
	fname = fname[pos+1:]

	info, err := os.Stat(fbase + "/" + fname)
	if err != nil {
		finishWithError(err)
	}

	html_byte, err := os.ReadFile(fbase + "/" + fname)
	if err != nil {
		finishWithError(err)
	}

	tracking[fbase+"/"+fname] = info.ModTime().Unix()
	html := strings.TrimSpace(string(html_byte))
	layout := ""

	for i, item := range depends {
		if fbase+"/"+fname == item {
			msg := "Recursive dependence cycle:\n"
			for i2, item2 := range depends {
				sep := "   "
				if i == i2 {
					sep = "-> "
				}
				msg += sep + item2
			}
			msg += "=> " + fbase + "/" + fname
			err := errors.New(msg)
			finishWithError(err)
		}
	}
	depends = append(depends, fbase+"/"+fname)

	re := regexp.MustCompile(`{{@([^}])+}}`)
	html = re.ReplaceAllStringFunc(html, func(match string) string {
		// remove {{@ and }}
		fragment := strings.TrimSpace(match[3 : len(match)-2])
		parts := strings.Fields(fragment)

		switch parts[0] {
		case "layout":
			layout = "templates/layouts/" + parts[1] + ".html"
			return ""

		case "content":
			return content

		case "css":
			fmt.Println("  @css", fbase+":", parts[1])
			return "<link rel=\"stylesheet\" href=\"../../" + parts[1] + "\" />"

		case "js":
			fmt.Println("  @js", fbase+":", parts[1])
			return "<script src=\"../../" + parts[1] + "\"></script>"

		case "global":
			fmt.Println("  @global", parts[1])
			return parse("templates/shared/"+parts[0]+".html", "", depends)

		case "local":
			fmt.Println("  @local", fbase+":", parts[1])
			return parse(fbase+"/"+parts[1]+".html", "", depends)

		default:
			return match // keep unchanged
		}
	})

	if layout == "" {
		return html
	}

	return parse(layout, html, depends)
}

func mustRebuild(fpath string) bool {
	fname := fpath + "/" + track_name

	_, err1 := os.Stat(fname)
	if err1 != nil {
		return true
	}

	data, err2 := os.ReadFile(fname)
	if err2 != nil {
		return true
	}

	err3 := json.Unmarshal(data, &tracking)
	if err3 != nil {
		return true
	}

	for fname, ftime := range tracking {
		if info, err := os.Stat(fname); err != nil || info.ModTime().Unix() != ftime {
			return true
		}
	}

	return false
}

func finishWithError(err error) {
	os.RemoveAll("ui/pages")
	panic(err)
}
