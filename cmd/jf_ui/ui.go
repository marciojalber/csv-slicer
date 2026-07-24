package main

// Validate {{@s...}}

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

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type UI struct {
	res *embed.FS
}

type Node struct {
	Type       string
	Attributes map[string]string
	Children   []*Node
}

var (
	ui         *UI
	basepath   = "templates/pages/"
	tracking   map[string]int64
	track_name = "__tracking.json"
	uiData     *hclsyntax.Body
)

func main() {
	getData()
	old_path := "ui/pages_old_" + strconv.FormatInt(time.Now().Unix(), 36)
	os.RemoveAll("ui/_pages")
	os.Mkdir("ui/_pages", 0644)
	parseDir("")
	os.Rename("ui/pages", old_path)
	os.Rename("ui/_pages", "ui/pages")
	os.RemoveAll(old_path)
}

func getData() {
	data, _ := os.ReadFile("templates/config.hcl")

	file, diags := hclsyntax.ParseConfig(
		data,
		"config.hcl",
		hcl.Pos{
			Line:   1,
			Column: 1,
		},
	)

	if diags.HasErrors() {
		panic(diags)
	}

	uiData = file.Body.(*hclsyntax.Body)

	/*
		for _, block := range uiData.Blocks {
			node := parseBlock(block)
			printNode(node, 0)
		}
	*/
}

func parseBlock(block *hclsyntax.Block) *Node {
	node := &Node{
		Type:       block.Type,
		Attributes: map[string]string{},
	}

	// Read attributes
	for name, attr := range block.Body.Attributes {
		// Simplified: only literal values
		val, _ := attr.Expr.Value(nil)

		node.Attributes[name] = val.AsString()
	}

	// Read child blocks
	for _, child := range block.Body.Blocks {
		node.Children = append(
			node.Children,
			parseBlock(child),
		)
	}

	return node
}

func printNode(n *Node, level int) {
	fmt.Printf(
		"%s%s %+v\n",
		string(make([]byte, level*2)),
		n.Type,
		n.Attributes,
	)

	for _, child := range n.Children {
		printNode(child, level+1)
	}
}

func parseData(path []string, block *hclsyntax.Block, default_val string) string {
	len_path := len(path)

	if block == nil {
		if len(path) < 2 {
			finishWithError(errors.New("Expected context[.context*].attribute"))
		}

		found := false
		for _, item := range uiData.Blocks {
			if item.Type == path[0] {
				block = item
				found = true
				break
			}
		}

		if !found {
			return default_val
		}

		path = path[1:]
		len_path--
	}

	if len_path == 1 {
		for key, attr := range block.Body.Attributes {
			if key != path[0] {
				continue
			}
			val, _ := attr.Expr.Value(nil)
			return val.AsString()
		}
		return default_val
	}

	if len(block.Body.Blocks) == 0 {
		return default_val
	}

	for _, node := range block.Body.Blocks {
		if node.Type != path[0] {
			continue
		}
		return parseData(path[1:], node, default_val)
	}

	return default_val
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

		if !mustRebuild(path) {
			fmt.Println("  File [ui/pages/" + path + ".html] is updated.")
			os.Link("ui/pages/"+path+".html", "ui/_pages/"+path+".html")
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

		case "favicon":
			fmt.Println("  @"+parts[0], fbase+":", parts[1])
			return "<link rel=\"icon\" href=\"../" + parts[1] + "\" type=\"" + parts[2] + "\"/>"

		case "data":
			fmt.Println("  @"+parts[0], parts[1], parts[2])
			return parseData(strings.Split(parts[1], "."), nil, parts[2])

		case "css":
			fmt.Println("  @"+parts[0], fbase+":", parts[1])
			return "<link rel=\"stylesheet\" href=\"../" + parts[1] + "\" />"

		case "js":
			fmt.Println("  @"+parts[0], fbase+":", parts[1])
			return "<script src=\"../" + parts[1] + "\"></script>"

		case "global":
			fmt.Println("  @"+parts[0], parts[1])
			return parse("templates/shared/"+parts[0]+".html", "", depends)

		case "local":
			fmt.Println("  @"+parts[0], fbase+":", parts[1])
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
	fname := basepath + fpath + "/" + track_name

	fhtml := "ui/pages/" + fpath + ".html"
	_, err1 := os.Stat(fhtml)
	if err1 != nil {
		return true
	}

	_, err2 := os.Stat(fname)
	if err2 != nil {
		return true
	}

	data, err3 := os.ReadFile(fname)
	if err3 != nil {
		return true
	}

	err4 := json.Unmarshal(data, &tracking)
	if err4 != nil {
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
