package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"math"

	"github.com/drhodes/golorem"
	"github.com/jaytaylor/html2text"
)

// zs var <filename> -- returns list of variables and their values
// zs var <filename> <var...> -- returns list of variable values
func Var(args ...string) string {
	if len(args) == 0 {
		return "var: filename expected"
	} else {
		s := ""
		if vars, _, err := md(args[0], globals()); err != nil {
			return "var: " + err.Error()
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
		return strings.TrimSpace(s)
	}
}

// zs lorem <n> -- returns <n> random lorem ipsum sentences
func Lorem(args ...string) string {
	if len(args) > 1 {
		return "lorem: invalid usage"
	}
	if len(args) == 0 {
		return lorem.Paragraph(5, 5)
	}
	if n, err := strconv.Atoi(args[0]); err == nil {
		return lorem.Paragraph(n, n)
	} else {
		return "lorem: " + err.Error()
	}
}

// zs datefmt <fmt> <date> -- returns formatted date from unix time
func DateFmt(args ...string) string {
	if len(args) == 0 || len(args) > 2 {
		return "datefmt: invalid usage"
	}
	if n, err := strconv.ParseInt(args[1], 10, 64); err == nil {
		return time.Unix(n, 0).Format(args[0])
	} else {
		return "datefmt: " + err.Error()
	}
}

// zs dateparse <fmt> <date> -- returns unix time from the formatted date
func DateParse(args ...string) string {
	if len(args) == 0 || len(args) > 2 {
		return "dateparse: invalid usage"
	}
	if d, err := time.Parse(args[0], args[1]); err != nil {
		return "dateparse: " + err.Error()
	} else {
		return strconv.FormatInt(d.Unix(), 10)
	}
}

// zs wc <file> -- returns word count in the file (markdown, html or amber)
func WordCount(args ...string) int {
	if os.Getenv("ZS_RECURSION") != "" {
		return 0
	}
	if len(args) != 1 {
		return 0
	}
	os.Setenv("ZS_RECURSION", "1")
	out := &bytes.Buffer{}
	if err := build(args[0], out, builtins(), globals()); err != nil {
		return 0
	}
	if s, err := html2text.FromString(string(out.Bytes())); err != nil {
		return 0
	} else {
		return len(strings.Fields(s))
	}
}

// zs timetoread <file> -- returns number of minutes required to read the text
func TimeToRead(args ...string) int {
	wc := WordCount(args...)
	return int(math.Floor(float64(wc)/200.0 + .5))
}

// zs ls <dir> <regexp>
func List(args ...string) []string {
	if len(args) != 2 {
		return []string{}
	}

	dir := args[0]
	mask := args[1]

	res := []string{}
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			if ok, err := filepath.Match(mask, info.Name()); ok && err == nil {
				res = append(res, path)
			}
		}
		return nil
	})
	return res
}

// zs sort <key> <files...>
func Sort(args ...string) []string {
	delim := -1
	for i, s := range args {
		if s == "--" {
			delim = i
		}
	}
	cmd := []string{"var", "title"}
	if delim != -1 {
		cmd = args[:delim]
		args = args[delim+1:]
	}

	sorted := map[string][]string{}
	sortedKeys := []string{}
	for _, f := range args {
		params := append(cmd, f)
		out := bytes.NewBuffer(nil)
		run(os.Args[0], params, globals(), out)
		val := string(out.Bytes())
		sorted[val] = append(sorted[val], f)
		sortedKeys = append(sortedKeys, val)
	}
	log.Println(sortedKeys)
	sort.Strings(sortedKeys)
	if !asc {
	}

	list := []string{}
	for _, k := range sortedKeys {
		vals := sorted[k]
		sort.Strings(vals)
		list = append(list, vals...)
	}
	return list
}
