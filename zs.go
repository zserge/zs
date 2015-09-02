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

func renameExt(path, from, to string) string {
	if from == "" {
		from = filepath.Ext(path)
	}
	if strings.HasSuffix(path, from) {
		return strings.TrimSuffix(path, from) + to
	} else {
		return path
	}
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
		"title":       "",
		"description": "",
		"keywords":    "",
	}
	for name, value := range globals {
		v[name] = value
	}
	if _, err := os.Stat(filepath.Join(ZSDIR, "layout.amber")); err == nil {
		v["layout"] = "layout.amber"
	} else {
		v["layout"] = "layout.html"
	}
	v["file"] = path
	v["url"] = url
	v["output"] = filepath.Join(PUBDIR, url)

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
func render(s string, vars Vars) (string, error) {
	tmpl, err := template.New("").Parse(s)
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
func buildMarkdown(path string, w io.Writer, vars Vars) error {
	v, body, err := md(path, vars)
	if err != nil {
		return err
	}
	content, err := render(body, v)
	if err != nil {
		return err
	}
	v["content"] = string(blackfriday.MarkdownCommon([]byte(content)))
	if w == nil {
		out, err := os.Create(filepath.Join(PUBDIR, renameExt(path, "", ".html")))
		if err != nil {
			return err
		}
		defer out.Close()
		w = out
	}
	if strings.HasSuffix(v["layout"], ".amber") {
		return buildAmber(filepath.Join(ZSDIR, v["layout"]), w, v)
	} else {
		return buildHTML(filepath.Join(ZSDIR, v["layout"]), w, v)
	}
}

// Renders text file expanding all variable macros inside it
func buildHTML(path string, w io.Writer, vars Vars) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	content, err := render(string(b), vars)
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
func buildAmber(path string, w io.Writer, vars Vars) error {
	a := amber.New()
	err := a.ParseFile(path)
	if err != nil {
		return err
	}

	data := map[string]interface{}{}
	for k, v := range vars {
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

func build(path string, w io.Writer, vars Vars) error {
	ext := filepath.Ext(path)
	if ext == ".md" || ext == ".mkd" {
		return buildMarkdown(path, w, vars)
	} else if ext == ".html" || ext == ".xml" {
		return buildHTML(path, w, vars)
	} else if ext == ".amber" {
		return buildAmber(path, w, vars)
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
		err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
			// ignore hidden files and directories
			if filepath.Base(path)[0] == '.' || strings.HasPrefix(path, ".") {
				return nil
			}
			// inform user about fs walk errors, but continue iteration
			if err != nil {
				log.Println("ERROR:", err)
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
				return build(path, nil, vars)
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
			if err := build(args[0], os.Stdout, globals()); err != nil {
				fmt.Println("ERROR: " + err.Error())
			}
		} else {
			fmt.Println("ERROR: too many arguments")
		}
	case "watch":
		buildAll(true)
	case "var":
		if len(args) == 0 {
			fmt.Println("var: filename expected")
		} else {
			s := ""
			if vars, _, err := md(args[0], globals()); err != nil {
				fmt.Println("var: " + err.Error())
			} else {
				if len(args) > 1 {
					for _, a := range args[1:] {
						s = s + vars[a] + "\n"
					}
				} else {
					for k, v := range vars {
						s = s + k + ":" + v + "\n"
					}
				}
			}
			fmt.Println(strings.TrimSpace(s))
		}
	default:
		err := run(path.Join(ZSDIR, cmd), args, globals(), os.Stdout)
		if err != nil {
			log.Println("ERROR:", err)
		}
	}
}
