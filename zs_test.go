package main

import "testing"

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

func TestMD(t *testing.T) {
	v, body := md(`
	title: Hello, world!
	keywords: foo, bar, baz
	empty:
	bayan: [:|||:]

this: is a content`)
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
	v, body = md("")
	if len(v) != 0 || len(body) != 0 {
		t.Error(v, body)
	}

	// Test empty header
	v, body = md("Hello")
	if len(v) != 0 || body != "Hello" {
		t.Error(v, body)
	}
}

func TestRender(t *testing.T) {
	eval := func(a []string, vars map[string]string) (string, error) {
		return "hello", nil
	}
	vars := map[string]string{"foo": "bar"}

	if s, err := render("plain text", vars, eval); err != nil || s != "plain text" {
		t.Error()
	}
	if s, err := render("a {{greet}} text", vars, eval); err != nil || s != "a hello text" {
		t.Error()
	}
	if s, err := render("{{greet}} x{{foo}}z", vars, eval); err != nil || s != "hello xbarz" {
		t.Error()
	}
}
