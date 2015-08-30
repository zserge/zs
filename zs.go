package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/eknkc/amber"
	"github.com/russross/blackfriday"
	"github.com/yosssi/gcss"
)

const (
	ZSDIR  = ".zs"
	PUBDIR = ".pub"
)

type Vars map[string]string
type Funcs template.FuncMap

// Parses markdown content. Returns parsed header variables and content
func md(path string, globals Vars) (Vars, string, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	s := string(b)
	url := path[:len(path)-len(filepath.Ext(path))] + ".html"
	v := Vars{
		"file":        path,
		"url":         url,
		"title":       "",
		"description": "",
		"keywords":    "",
		"output":      filepath.Join(PUBDIR, url),
	}
	if _, err := os.Stat(filepath.Join(ZSDIR, "layout.amber")); err == nil {
		v["layout"] = "layout.amber"
	} else {
		v["layout"] = "layout.html"
	}

	for name, value := range globals {
		v[name] = value
	}
	if strings.Index(s, "\n\n") == -1 {
		return v, s, nil
	}
	header, body := split2(s, "\n\n")
	for _, line := range strings.Split(header, "\n") {
		key, value := split2(line, ":")
		v[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	if strings.HasPrefix(v["url"], "./") {
		v["url"] = v["url"][2:]
	}
	return v, body, nil
}

// Use standard Go templates
func render(s string, funcs Funcs, vars Vars) (string, error) {
	f := Funcs{}
	for k, v := range funcs {
		f[k] = v
	}
	for k, v := range vars {
		f[k] = varFunc(v)
	}
	// Plugin functions
	files, _ := ioutil.ReadDir(ZSDIR)
	for _, file := range files {
		if !file.IsDir() {
			name := file.Name()
			if !strings.HasSuffix(name, ".html") && !strings.HasSuffix(name, ".amber") {
				f[strings.TrimSuffix(name, filepath.Ext(name))] = pluginFunc(name, vars)
			}
		}
	}

	tmpl, err := template.New("").Funcs(template.FuncMap(f)).Parse(s)
	if err != nil {
		return "", err
	}
	out := &bytes.Buffer{}
	if err := tmpl.Execute(out, vars); err != nil {
		return "", err
	}
	return string(out.Bytes()), nil
}

// Renders markdown with the given layout into html expanding all the macros
func buildMarkdown(path string, w io.Writer, funcs Funcs, vars Vars) error {
	v, body, err := md(path, vars)
	if err != nil {
		return err
	}
	content, err := render(body, funcs, v)
	if err != nil {
		return err
	}
	v["content"] = string(blackfriday.MarkdownBasic([]byte(content)))
	if w == nil {
		out, err := os.Create(filepath.Join(PUBDIR, renameExt(path, "", ".html")))
		if err != nil {
			return err
		}
		defer out.Close()
		w = out
	}
	if strings.HasSuffix(v["layout"], ".amber") {
		return buildAmber(filepath.Join(ZSDIR, v["layout"]), w, funcs, v)
	} else {
		return buildHTML(filepath.Join(ZSDIR, v["layout"]), w, funcs, v)
	}
}

// Renders text file expanding all variable macros inside it
func buildHTML(path string, w io.Writer, funcs Funcs, vars Vars) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	content, err := render(string(b), funcs, vars)
	if err != nil {
		return err
	}
	if w == nil {
		f, err := os.Create(filepath.Join(PUBDIR, path))
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	_, err = io.WriteString(w, content)
	return err
}

// Renders .amber file into .html
func buildAmber(path string, w io.Writer, funcs Funcs, vars Vars) error {
	a := amber.New()
	err := a.ParseFile(path)
	if err != nil {
		return err
	}

	data := map[string]interface{}{}
	for k, v := range vars {
		data[k] = v
	}
	for k, v := range funcs {
		data[k] = v
	}

	t, err := a.Compile()
	if err != nil {
		return err
	}
	if w == nil {
		f, err := os.Create(filepath.Join(PUBDIR, renameExt(path, ".amber", ".html")))
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	return t.Execute(w, data)
}

// Compiles .gcss into .css
func buildGCSS(path string, w io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if w == nil {
		s := strings.TrimSuffix(path, ".gcss") + ".css"
		css, err := os.Create(filepath.Join(PUBDIR, s))
		if err != nil {
			return err
		}
		defer css.Close()
		w = css
	}
	_, err = gcss.Compile(w, f)
	return err
}

// Copies file as is from path to writer
func buildRaw(path string, w io.Writer) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	if w == nil {
		if out, err := os.Create(filepath.Join(PUBDIR, path)); err != nil {
			return err
		} else {
			defer out.Close()
			w = out
		}
	}
	_, err = io.Copy(w, in)
	return err
}

func build(path string, w io.Writer, funcs Funcs, vars Vars) error {
	ext := filepath.Ext(path)
	if ext == ".md" || ext == ".mkd" {
		return buildMarkdown(path, w, funcs, vars)
	} else if ext == ".html" || ext == ".xml" {
		return buildHTML(path, w, funcs, vars)
	} else if ext == ".amber" {
		return buildAmber(path, w, funcs, vars)
	} else if ext == ".gcss" {
		return buildGCSS(path, w)
	} else {
		return buildRaw(path, w)
	}
}

func buildAll(watch bool) {
	lastModified := time.Unix(0, 0)
	modified := false

	vars := globals()
	for {
		os.Mkdir(PUBDIR, 0755)
		funcs := builtins()
		err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
			// ignore hidden files and directories
			if filepath.Base(path)[0] == '.' || strings.HasPrefix(path, ".") {
				return nil
			}

			if info.IsDir() {
				os.Mkdir(filepath.Join(PUBDIR, path), 0755)
				return nil
			} else if info.ModTime().After(lastModified) {
				if !modified {
					// About to be modified, so run pre-build hook
					// FIXME on windows it might not work well
					run(filepath.Join(ZSDIR, "pre"), []string{}, nil, nil)
					modified = true
				}
				log.Println("build: ", path)
				return build(path, nil, funcs, vars)
			}
			return nil
		})
		if err != nil {
			log.Println("ERROR:", err)
		}
		if modified {
			// Something was modified, so post-build hook
			// FIXME on windows it might not work well
			run(filepath.Join(ZSDIR, "post"), []string{}, nil, nil)
			modified = false
		}
		if !watch {
			break
		}
		lastModified = time.Now()
		time.Sleep(1 * time.Second)
	}
}

func main() {
	if len(os.Args) == 1 {
		fmt.Println(os.Args[0], "<command> [args]")
		return
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "build":
		if len(args) == 0 {
			buildAll(false)
		} else if len(args) == 1 {
			if err := build(args[0], os.Stdout, builtins(), globals()); err != nil {
				fmt.Println("ERROR: " + err.Error())
			}
		} else {
			fmt.Println("ERROR: too many arguments")
		}
	case "watch":
		buildAll(true)
	case "var":
		fmt.Println(Var(args))
	case "lorem":
		fmt.Println(Lorem(args))
	case "dateparse":
		fmt.Println(DateParse(args))
	case "datefmt":
		fmt.Println(DateFmt(args))
	case "wc":
		fmt.Println(WordCount(args))
	case "timetoread":
		fmt.Println(TimeToRead(args))
	default:
		err := run(path.Join(ZSDIR, cmd), args, globals(), os.Stdout)
		if err != nil {
			log.Println("ERROR:", err)
		}
	}
}
