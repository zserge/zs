package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
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

// Splits a string in exactly two parts by delimiter
// If no delimiter is found - the second string is be empty
func split2(s, delim string) (string, string) {
	parts := strings.SplitN(s, delim, 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	} else {
		return parts[0], ""
	}
}

// Parses markdown content. Returns parsed header variables and content
func md(path string, globals Vars) (Vars, string, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	s := string(b)
	url := path[:len(path)-len(filepath.Ext(path))] + ".html"
	v := Vars{
		"file":   path,
		"url":    url,
		"output": filepath.Join(PUBDIR, url),
	}
	if _, err := os.Stat(filepath.Join(ZSDIR, "layout.amber")); err == nil {
		v["layout"] = "layout.amber"
	} else {
		v["layout"] = "layout.html"
	}

	if info, err := os.Stat(path); err == nil {
		v["date"] = info.ModTime().Format("02-01-2006")
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
func render(s string, funcs template.FuncMap, vars Vars) (string, error) {
	f := template.FuncMap{}
	for k, v := range funcs {
		f[k] = v
	}
	for k, v := range vars {
		f[k] = varFunc(v)
	}
	tmpl, err := template.New("").Funcs(f).Parse(s)
	if err != nil {
		return "", err
	}
	out := &bytes.Buffer{}
	if err := tmpl.Execute(out, vars); err != nil {
		return "", err
	}
	return string(out.Bytes()), nil
}

// Converts zs markdown variables into environment variables
func env(vars Vars) []string {
	env := []string{"ZS=" + os.Args[0], "ZS_OUTDIR=" + PUBDIR}
	env = append(env, os.Environ()...)
	if vars != nil {
		for k, v := range vars {
			env = append(env, "ZS_"+strings.ToUpper(k)+"="+v)
		}
	}
	return env
}

// Runs command with given arguments and variables, intercepts stderr and
// redirects stdout into the given writer
func run(cmd string, args []string, vars Vars, output io.Writer) error {
	var errbuf bytes.Buffer
	c := exec.Command(cmd, args...)
	c.Env = env(vars)
	c.Stdout = output
	c.Stderr = &errbuf

	err := c.Run()

	if errbuf.Len() > 0 {
		log.Println("ERROR:", errbuf.String())
	}

	if err != nil {
		return err
	}
	return nil
}

// Expands macro: either replacing it with the variable value, or
// running the plugin command and replacing it with the command's output
func eval(cmd []string, vars Vars) (string, error) {
	outbuf := bytes.NewBuffer(nil)
	err := run(path.Join(ZSDIR, cmd[0]), cmd[1:], vars, outbuf)
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return "", err
		}
		outbuf = bytes.NewBuffer(nil)
		err := run(cmd[0], cmd[1:], vars, outbuf)
		// Return exit errors, but ignore if the command was not found
		if _, ok := err.(*exec.ExitError); ok {
			return "", err
		}
	}
	return outbuf.String(), nil
}

// Renders markdown with the given layout into html expanding all the macros
func buildMarkdown(path string, funcs template.FuncMap, vars Vars) error {
	v, body, err := md(path, vars)
	if err != nil {
		return err
	}
	content, err := render(body, funcs, v)
	if err != nil {
		return err
	}
	v["content"] = string(blackfriday.MarkdownBasic([]byte(content)))
	if strings.HasSuffix(v["layout"], ".amber") {
		return buildAmber(filepath.Join(ZSDIR, v["layout"]),
			renameExt(path, "", ".html"), funcs, v)
	} else {
		return buildPlain(filepath.Join(ZSDIR, v["layout"]),
			renameExt(path, "", ".html"), funcs, v)
	}
}

// Renders text file expanding all variable macros inside it
func buildPlain(in, out string, funcs template.FuncMap, vars Vars) error {
	b, err := ioutil.ReadFile(in)
	if err != nil {
		return err
	}
	content, err := render(string(b), funcs, vars)
	if err != nil {
		return err
	}
	output := filepath.Join(PUBDIR, out)
	if s, ok := vars["output"]; ok {
		output = s
	}
	err = ioutil.WriteFile(output, []byte(content), 0666)
	if err != nil {
		return err
	}
	return nil
}

// Renders .amber file into .html
func buildAmber(in, out string, funcs template.FuncMap, vars Vars) error {
	a := amber.New()
	err := a.ParseFile(in)
	if err != nil {
		return err
	}
	t, err := a.Compile()
	if err != nil {
		return err
	}
	//amber.FuncMap = amber.FuncMap
	f, err := os.Create(filepath.Join(PUBDIR, out))
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, vars)
}

// Compiles .gcss into .css
func buildGCSS(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	s := strings.TrimSuffix(path, ".gcss") + ".css"
	css, err := os.Create(filepath.Join(PUBDIR, s))
	if err != nil {
		return err
	}

	defer f.Close()
	defer css.Close()

	_, err = gcss.Compile(css, f)
	return err
}

// Copies file from working directory into public directory
func copyFile(path string) (err error) {
	var in, out *os.File
	if in, err = os.Open(path); err == nil {
		defer in.Close()
		if out, err = os.Create(filepath.Join(PUBDIR, path)); err == nil {
			defer out.Close()
			_, err = io.Copy(out, in)
		}
	}
	return err
}

func varFunc(s string) func() string {
	return func() string {
		return s
	}
}

func pluginFunc(cmd string) func() string {
	return func() string {
		return "Not implemented yet"
	}
}

func createFuncs() template.FuncMap {
	// Builtin functions
	funcs := template.FuncMap{
		"exec": func(s ...string) string {
			// Run external command with arguments
			return ""
		},
		"zs": func(args ...string) string {
			// Run zs with arguments
			return ""
		},
	}
	// Plugin functions
	files, _ := ioutil.ReadDir(ZSDIR)
	for _, f := range files {
		if !f.IsDir() {
			name := f.Name()
			if !strings.HasSuffix(name, ".html") && !strings.HasSuffix(name, ".amber") {
				funcs[strings.TrimSuffix(name, filepath.Ext(name))] = pluginFunc(name)
			}
		}
	}
	return funcs
}

func renameExt(path, from, to string) string {
	if from == "" {
		from = filepath.Ext(path)
	}
	return strings.TrimSuffix(path, from) + to
}

func globals() Vars {
	vars := Vars{}
	for _, e := range os.Environ() {
		pair := strings.Split(e, "=")
		if strings.HasPrefix(pair[0], "ZS_") {
			vars[strings.ToLower(pair[0][3:])] = pair[1]
		}
	}
	return vars
}

func buildAll(once bool) {
	lastModified := time.Unix(0, 0)
	modified := false

	vars := globals()
	for {
		os.Mkdir(PUBDIR, 0755)
		funcs := createFuncs()
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
				ext := filepath.Ext(path)
				if ext == ".md" || ext == ".mkd" {
					log.Println("md: ", path)
					return buildMarkdown(path, funcs, vars)
				} else if ext == ".html" || ext == ".xml" {
					log.Println("html: ", path)
					return buildPlain(path, path, funcs, vars)
				} else if ext == ".amber" {
					log.Println("html: ", path)
					return buildAmber(path, renameExt(path, ".amber", ".html"), funcs, vars)
				} else if ext == ".gcss" {
					log.Println("css: ", path)
					return buildGCSS(path)
				} else {
					log.Println("raw: ", path)
					return copyFile(path)
				}
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
		lastModified = time.Now()
		if once {
			break
		}
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
		buildAll(true)
	case "watch":
		buildAll(false) // pass duration
	case "var":
		if len(args) == 0 {
			log.Println("ERROR: filename expected")
			return
		}
		if vars, _, err := md(args[0], globals()); err == nil {
			if len(args) > 1 {
				for _, a := range args[1:] {
					fmt.Println(vars[a])
				}
			} else {
				for k, v := range vars {
					fmt.Println(k + ":" + v)
				}
			}
		} else {
			log.Println("ERROR:", err)
		}
	default:
		err := run(path.Join(ZSDIR, cmd), args, Vars{}, os.Stdout)
		if err != nil {
			log.Println("ERROR:", err)
		}
	}
}
