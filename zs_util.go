package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func makeFuncs(funcs Funcs, vars Vars) Funcs {
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
				f[renameExt(name, "", "")] = pluginFunc(name, vars)
			} else {
				f[renameExt(name, "", "")] = partialFunc(name, f, vars)
			}
		}
	}
	return f
}

func varFunc(s string) func() string {
	return func() string {
		return s
	}
}

func pluginFunc(cmd string, vars Vars) func(args ...string) string {
	return func(args ...string) string {
		out := bytes.NewBuffer(nil)
		if err := run(filepath.Join(ZSDIR, cmd), args, vars, out); err != nil {
			return cmd + ":" + err.Error()
		} else {
			return string(out.Bytes())
		}
	}
}

func partialFunc(name string, funcs Funcs, vars Vars) func() string {
	return func() string {
		var err error
		w := bytes.NewBuffer(nil)
		if strings.HasSuffix(name, ".amber") {
			err = buildAmber(filepath.Join(ZSDIR, name), w, funcs, vars)
		} else {
			err = buildHTML(filepath.Join(ZSDIR, name), w, funcs, vars)
		}
		if err != nil {
			return name + ":" + err.Error()
		}
		return string(w.Bytes())
	}
}

func builtins() Funcs {
	exec := func(cmd string, args ...string) string {
		out := bytes.NewBuffer(nil)
		if err := run(cmd, args, Vars{}, out); err != nil {
			return cmd + ":" + err.Error()
		} else {
			return string(out.Bytes())
		}
		return ""
	}
	return Funcs{
		"exec":      exec,
		"var":       Var,
		"lorem":     Lorem,
		"dateparse": DateParse,
		"datefmt":   DateFmt,
		"wc":        WordCount,
		"ttr":       TimeToRead,
		"ls":        List,
		"...": func(args ...string) []string {
			return append([]string{"..."}, args...)
		},
		"sort": func(args ...string) []string {

			return Sort(args...)
		},
	}
}

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
