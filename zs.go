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

	"github.com/russross/blackfriday"
)

const (
	ZSDIR  = ".zs"
	PUBDIR = ".pub"
)

type EvalFn func(args []string, vars map[string]string) (string, error)

func split2(s, delim string) (string, string) {
	parts := strings.SplitN(s, delim, 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	} else {
		return parts[0], ""
	}
}

func md(path, s string) (map[string]string, string) {
	url := path[:len(path)-len(filepath.Ext(path))] + ".html"
	v := map[string]string{
		"file":   path,
		"url":    url,
		"output": filepath.Join(PUBDIR, url),
		"layout": "index.html",
	}
	if strings.Index(s, "\n\n") == -1 {
		return map[string]string{}, s
	}
	header, body := split2(s, "\n\n")
	for _, line := range strings.Split(header, "\n") {
		key, value := split2(line, ":")
		v[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	if strings.HasPrefix(v["url"], "./") {
		v["url"] = v["url"][2:]
	}
	return v, body
}

func render(s string, vars map[string]string, eval EvalFn) (string, error) {
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

func env(vars map[string]string) []string {
	env := []string{"ZS=" + os.Args[0], "ZS_OUTDIR=" + PUBDIR}
	env = append(env, os.Environ()...)
	if vars != nil {
		for k, v := range vars {
			env = append(env, "ZS_"+strings.ToUpper(k)+"="+v)
		}
	}
	return env
}

func run(cmd string, args []string, vars map[string]string, output io.Writer) error {
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

func eval(cmd []string, vars map[string]string) (string, error) {
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
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	v, body := md(path, string(b))
	content, err := render(body, v, eval)
	if err != nil {
		return err
	}
	v["content"] = string(blackfriday.MarkdownBasic([]byte(content)))
	return buildPlain(filepath.Join(ZSDIR, v["layout"]), v)
}

func buildPlain(path string, vars map[string]string) error {
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
					log.Println("mkd: ", path)
					return buildMarkdown(path)
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
		if b, err := ioutil.ReadFile(args[0]); err == nil {
			vars, _ := md(args[0], string(b))
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
		err := run(path.Join(ZSDIR, cmd), args, map[string]string{}, os.Stdout)
		if err != nil {
			log.Println(err)
		}
	}
}
