package main

import (
	"embed"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type UI struct {
	res *embed.FS
}

var (
	ui       *UI
	basepath = "templates/pages/"
)

func main() {
	os.RemoveAll("ui/pages")
	os.Mkdir("ui/pages", 0644)
	parseDir("")
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

		MakePage(path)
	}
}

func MakePage(fname string) string {
	pos := strings.LastIndex(fname, "/")
	dir := "ui/pages"
	if pos != -1 {
		dir += "/" + fname[:pos]
	}
	pname := fname[pos+1:] + ".html"
	if err := os.MkdirAll(dir, 0644); err != nil {
		panic(err)
	}

	depends := []string{}
	html := parse("templates/pages/"+fname+"/__index.html", "", depends)
	os.WriteFile(dir+"/"+pname, []byte(html), 0644)

	return html
}

func parse(fname, content string, depends []string) string {
	pos := strings.LastIndex(fname, "/")
	fbase := fname[:pos]
	fname = fname[pos+1:]

	html_byte, err := os.ReadFile(fbase + "/" + fname)
	if err != nil {
		panic(err)
	}
	html := strings.TrimSpace(string(html_byte))
	layout := ""

	for i, item := range depends {
		if fbase+"/"+fname == item {
			fmt.Println("\nRecursive dependence cycle:")
			for i2, item2 := range depends {
				sep := "  "
				if i == i2 {
					sep = "->"
				}
				fmt.Println(sep, item2)
			}
			fmt.Println("=>", fbase+"/"+fname)
			os.Exit(1)
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
			fmt.Println("@css", fbase+":", parts[1])
			return "<link rel=\"stylesheet\" href=\"../../" + parts[1] + "\" />"

		case "js":
			fmt.Println("@js", fbase+":", parts[1])
			return "<script src=\"../../" + parts[1] + "\"></script>"

		case "global":
			fmt.Println("@global", parts[1])
			return parse("templates/shared/"+parts[0]+".html", "", depends)

		case "local":
			fmt.Println("@local", fbase+":", parts[1])
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
