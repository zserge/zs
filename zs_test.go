package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
	"text/template"
)

func TestSplit2(t *testing.T) {
	if a, b := split2("a:b", ":"); a != "a" || b != "b" {
		t.Fail()
	}
	if a, b := split2(":b", ":"); a != "" || b != "b" {
		t.Fail()
	}
	if a, b := split2("a:", ":"); a != "a" || b != "" {
		t.Fail()
	}
	if a, b := split2(":", ":"); a != "" || b != "" {
		t.Fail()
	}
	if a, b := split2("a", ":"); a != "a" || b != "" {
		t.Fail()
	}
	if a, b := split2("", ":"); a != "" || b != "" {
		t.Fail()
	}
}

func tmpfile(path, s string) string {
	ioutil.WriteFile(path, []byte(s), 0644)
	return path
}

func TestMD(t *testing.T) {
	defer os.Remove("foo.md")
	v, body, _ := md(tmpfile("foo.md", `
	title: Hello, world!
	keywords: foo, bar, baz
	empty:
	bayan: [:|||:]

this: is a content`))
	if v["title"] != "Hello, world!" {
		t.Error()
	}
	if v["keywords"] != "foo, bar, baz" {
		t.Error()
	}
	if s, ok := v["empty"]; !ok || len(s) != 0 {
		t.Error()
	}
	if v["bayan"] != "[:|||:]" {
		t.Error()
	}
	if body != "this: is a content" {
		t.Error(body)
	}

	// Test empty md
	v, body, _ = md(tmpfile("foo.md", ""))
	if len(v) != 0 || len(body) != 0 {
		t.Error(v, body)
	}

	// Test empty header
	v, body, _ = md(tmpfile("foo.md", "Hello"))
	if len(v) != 0 || body != "Hello" {
		t.Error(v, body)
	}
}

func TestRender(t *testing.T) {
	vars := map[string]string{"foo": "bar"}
	funcs := template.FuncMap{
		"greet": func(s ...string) string {
			if len(s) == 0 {
				return "hello"
			} else {
				return "hello " + strings.Join(s, " ")
			}
		},
	}

	if s, err := render("plain text", funcs, vars); err != nil || s != "plain text" {
		t.Error(s, err)
	}
	if s, err := render("a {{greet}} text", funcs, vars); err != nil || s != "a hello text" {
		t.Error(s, err)
	}
	if s, err := render("{{greet}} x{{foo}}z", funcs, vars); err != nil || s != "hello xbarz" {
		t.Error(s, err)
	}
	// Test error case
	if s, err := render("a {{greet text ", funcs, vars); err == nil || len(s) != 0 {
		t.Error(s, err)
	}
}

func TestEnv(t *testing.T) {
	e := env(map[string]string{"foo": "bar", "baz": "hello world"})
	mustHave := []string{"ZS=" + os.Args[0], "ZS_FOO=bar", "ZS_BAZ=hello world", "PATH="}
	for _, s := range mustHave {
		found := false
		for _, v := range e {
			if strings.HasPrefix(v, s) {
				found = true
				break
			}
		}
		if !found {
			t.Error("Missing", s)
		}
	}
}

func TestRun(t *testing.T) {
	out := bytes.NewBuffer(nil)
	err := run("some_unbelievable_command_name", []string{}, map[string]string{}, out)
	if err == nil {
		t.Error()
	}

	out = bytes.NewBuffer(nil)
	err = run(os.Args[0], []string{"-test.run=TestHelperProcess"},
		map[string]string{"helper": "1", "out": "foo", "err": "bar"}, out)
	if err != nil {
		t.Error(err)
	}
	if out.String() != "foo\n" {
		t.Error(out.String())
	}
}

func TestEvalCommand(t *testing.T) {
	s, err := eval([]string{"echo", "hello"}, map[string]string{})
	if err != nil {
		t.Error(err)
	}
	if s != "hello\n" {
		t.Error(s)
	}
	_, err = eval([]string{"cat", "bogus/file"}, map[string]string{})
	if _, ok := err.(*exec.ExitError); !ok {
		t.Error("expected ExitError")
	}
	_, err = eval([]string{"missing command"}, map[string]string{})
	if err != nil {
		t.Error("missing command should be ignored")
	}
}

func TestHelperProcess(*testing.T) {
	if os.Getenv("ZS_HELPER") != "1" {
		return
	}
	defer os.Exit(0)                 // TODO check exit code
	log.Println(os.Getenv("ZS_ERR")) // stderr
	fmt.Println(os.Getenv("ZS_OUT")) // stdout
}
