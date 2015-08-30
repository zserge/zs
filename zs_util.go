package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func varFunc(s string) func() string {
	return func() string {
		return s
	}
}

func pluginFunc(cmd string, vars Vars) func(args ...string) string {
	return func(args ...string) string {
		out := bytes.NewBuffer(nil)
		if err := run(cmd, args, vars, out); err != nil {
			return cmd + ":" + err.Error()
		} else {
			return string(out.Bytes())
		}
	}
}

func builtins() Funcs {
	exec := func(s ...string) string {
		return ""
	}
	return Funcs{
		"exec": exec,
		"zs": func(args ...string) string {
			cmd := []string{"zs"}
			cmd = append(cmd, args...)
			return exec(cmd...)
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
