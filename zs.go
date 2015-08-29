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

type EvalFn func(args []string, vars Vars) (string, error)

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
func md(path string) (Vars, string, error) {
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
		"layout": "index.html",
	}
	if strings.Index(s, "\n\n") == -1 {
		return Vars{}, s, nil
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

func render(s string, vars Vars, eval EvalFn) (string, error) {
	delim_open := "{{"
	delim_close := "}}"

	out := bytes.NewBuffer(nil)
	for {
		if from := strings.Index(s, delim_open); from == -1 {
			out.WriteString(s)
			return out.String(), nil
		} else {
			if to := strings.Index(s, delim_close); to == -1 {
				return "", fmt.Errorf("Close delim not found")
			} else {
				out.WriteString(s[:from])
				cmd := s[from+len(delim_open) : to]
				s = s[to+len(delim_close):]
				m := strings.Fields(cmd)
				if len(m) == 1 {
					if v, ok := vars[m[0]]; ok {
						out.WriteString(v)
						continue
					}
				}
				if res, err := eval(m, vars); err == nil {
					out.WriteString(res)
				} else {
					log.Println(err) // silent
				}
			}
		}
	}
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
		log.Println(errbuf.String())
	}

	if err != nil {
		return err
	}
	return nil
}

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

func buildMarkdown(path string) error {
	v, body, err := md(path)
	if err != nil {
		return err
	}
	content, err := render(body, v, eval)
	if err != nil {
		return err
	}
	v["content"] = string(blackfriday.MarkdownBasic([]byte(content)))
	return buildPlain(filepath.Join(ZSDIR, v["layout"]), v)
}

func buildPlain(path string, vars Vars) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	content, err := render(string(b), vars, eval)
	if err != nil {
		return err
	}
	output := filepath.Join(PUBDIR, path)
	if s, ok := vars["output"]; ok {
		output = s
	}
	err = ioutil.WriteFile(output, []byte(content), 0666)
	if err != nil {
		return err
	}
	return nil
}

func buildGCSS(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	s := strings.TrimSuffix(path, ".gcss") + ".css"
	log.Println(s)
	css, err := os.Create(filepath.Join(PUBDIR, s))
	if err != nil {
		return err
	}

	defer f.Close()
	defer css.Close()

	_, err = gcss.Compile(css, f)
	return err
}

func buildAmber(path string, vars Vars) error {
	a := amber.New()
	err := a.ParseFile(path)
	if err != nil {
		return err
	}
	t, err := a.Compile()
	if err != nil {
		return err
	}
	//amber.FuncMap = amber.FuncMap
	s := strings.TrimSuffix(path, ".amber") + ".html"
	f, err := os.Create(filepath.Join(PUBDIR, s))
	if err != nil {
		return err
	}
	defer f.Close()
	return t.Execute(f, vars)
}

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

func buildAll(once bool) {
	lastModified := time.Unix(0, 0)
	modified := false
	for {
		os.Mkdir(PUBDIR, 0755)
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
					run(filepath.Join(ZSDIR, "pre"), []string{}, nil, nil)
					modified = true
				}
				ext := filepath.Ext(path)
				if ext == ".md" || ext == ".mkd" {
					log.Println("md: ", path)
					return buildMarkdown(path)
				} else if ext == ".html" || ext == ".xml" {
					log.Println("html: ", path)
					return buildPlain(path, Vars{})
				} else if ext == ".amber" {
					log.Println("html: ", path)
					return buildAmber(path, Vars{})
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
		if vars, _, err := md(args[0]); err == nil {
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
			log.Println(err)
		}
	default:
		err := run(path.Join(ZSDIR, cmd), args, Vars{}, os.Stdout)
		if err != nil {
			log.Println(err)
		}
	}
}
