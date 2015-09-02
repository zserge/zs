package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestRenameExt(t *testing.T) {
	if s := renameExt("foo.amber", ".amber", ".html"); s != "foo.html" {
		t.Error(s)
	}
	if s := renameExt("foo.amber", "", ".html"); s != "foo.html" {
		t.Error(s)
	}
	if s := renameExt("foo.amber", ".md", ".html"); s != "foo.amber" {
		t.Error(s)
	}
	if s := renameExt("foo", ".amber", ".html"); s != "foo" {
		t.Error(s)
	}
	if s := renameExt("foo", "", ".html"); s != "foo.html" {
		t.Error(s)
	}
}

func TestRun(t *testing.T) {
	// external command
	if s, err := run(Vars{}, "echo", "hello"); err != nil || s != "hello\n" {
		t.Error(s, err)
	}
	// passing variables to plugins
	if s, err := run(Vars{"foo": "bar"}, "sh", "-c", "echo $ZS_FOO"); err != nil || s != "bar\n" {
		t.Error(s, err)
	}

	// custom plugin overriding external command
	os.Mkdir(ZSDIR, 0755)
	script := `#!/bin/sh
echo foo
`
	ioutil.WriteFile(filepath.Join(ZSDIR, "echo"), []byte(script), 0755)
	if s, err := run(Vars{}, "echo", "hello"); err != nil || s != "foo\n" {
		t.Error(s, err)
	}
	os.Remove(filepath.Join(ZSDIR, "echo"))
	os.Remove(ZSDIR)
}

func TestVars(t *testing.T) {
	tests := map[string]Vars{
		`
foo: bar
title: Hello, world!
---
Some content in markdown
`: Vars{
			"foo":       "bar",
			"title":     "Hello, world!",
			"url":       "test.html",
			"file":      "test.md",
			"output":    filepath.Join(PUBDIR, "test.html"),
			"__content": "Some content in markdown\n",
		},
		`
url: "example.com/foo.html"
---
Hello
`: Vars{
			"url":       "example.com/foo.html",
			"__content": "Hello\n",
		},
	}

	for script, vars := range tests {
		ioutil.WriteFile("test.md", []byte(script), 0644)
		if v, s, err := getVars("test.md", Vars{"baz": "123"}); err != nil {
			t.Error(err)
		} else if s != vars["__content"] {
			t.Error(s, vars["__content"])
		} else {
			for key, value := range vars {
				if key != "__content" && v[key] != value {
					t.Error(key, v[key], value)
				}
			}
		}
	}
}

func TestRender(t *testing.T) {
	vars := map[string]string{"foo": "bar"}

	if s, _ := render("foo bar", vars); s != "foo bar" {
		t.Error(s)
	}
	if s, _ := render("a {{printf short}} text", vars); s != "a short text" {
		t.Error(s)
	}
	if s, _ := render("{{printf Hello}} x{{foo}}z", vars); s != "Hello xbarz" {
		t.Error(s)
	}
	// Test error case
	if _, err := render("a {{greet text ", vars); err == nil {
		t.Error("error expected")
	}
}
