package main

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const TESTDIR = ".test"

func TestBuild(t *testing.T) {
	files, _ := ioutil.ReadDir("testdata")
	for _, f := range files {
		if f.IsDir() {
			testBuild(filepath.Join("testdata", f.Name()), t)
		}
	}
}

func testBuild(path string, t *testing.T) {
	wd, _ := os.Getwd()
	os.Chdir(path)
	args := os.Args[:]
	os.Args = []string{"zs", "build"}
	t.Log("--- BUILD", path)
	main()

	compare(PUBDIR, TESTDIR, t)

	os.Chdir(wd)
	os.Args = args
}

func compare(pub, test string, t *testing.T) {
	a := md5dir(pub)
	b := md5dir(test)
	for k, v := range a {
		if s, ok := b[k]; !ok {
			t.Error("Unexpected file:", k, v)
		} else if s != v {
			t.Error("Different file:", k, v, s)
		} else {
			t.Log("Matching file", k, v)
		}
	}
	for k, v := range b {
		if _, ok := a[k]; !ok {
			t.Error("Missing file:", k, v)
		}
	}
}

func md5dir(path string) map[string]string {
	files := map[string]string{}
	filepath.Walk(path, func(s string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			if f, err := os.Open(s); err == nil {
				defer f.Close()
				hash := md5.New()
				io.Copy(hash, f)
				files[strings.TrimPrefix(s, path)] = hex.EncodeToString(hash.Sum(nil))
			}
		}
		return nil
	})
	return files
}
