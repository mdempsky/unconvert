// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestBinary(t *testing.T) {
	exepath, cleanup := build(t)
	defer cleanup()

	output, err := exec.Command(exepath, "./testdata").CombinedOutput()
	if err == nil {
		t.Fatal("expected to quit with an error code")
	}

	got, err := ParseOutput("testdata", string(output))
	if err != nil {
		t.Fatal(err)
	}

	expected, err := ParseDir("testdata")
	if err != nil {
		t.Fatal(err)
	}

	SortAnnotations(got)
	SortAnnotations(expected)

	if len(got) != len(expected) {
		t.Errorf("different number of results: got %v expected %v", len(got), len(expected))
	}

	n := len(got)
	if len(expected) < n {
		n = len(expected)
	}
	for i, a := range got[:n] {
		b := expected[i]
		if a != b {
			t.Errorf("got %q expected %q", a, b)
		}
	}
}

type Annotation struct {
	File    string
	Line    int
	Message string
}

func (ann Annotation) String() string {
	return fmt.Sprintf("%s:%d:0: %s", ann.File, ann.Line, ann.Message)
}

func SortAnnotations(xs []Annotation) {
	sort.Slice(xs, func(i, k int) bool {
		if xs[i].File == xs[k].File {
			return xs[i].Line < xs[k].Line
		}
		return xs[i].File < xs[k].File
	})
}

func ParseOutput(folder, output string) ([]Annotation, error) {
	var all []Annotation
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		folderStart := strings.Index(line, folder)
		if folderStart < 0 {
			continue
		}

		line = line[folderStart+len(folder)+1:]
		tokens := strings.SplitN(line, ":", 4)
		if len(tokens) != 4 {
			continue
		}

		line, err := strconv.Atoi(tokens[1])
		if err != nil {
			return nil, err
		}

		all = append(all, Annotation{
			File:    tokens[0],
			Line:    line,
			Message: strings.TrimSpace(tokens[3]),
		})
	}
	return all, nil
}

func ParseDir(dir string) ([]Annotation, error) {
	var all []Annotation
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		xs, err := ParseFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return all, err
		}
		all = append(all, xs...)
	}

	return all, nil
}

func ParseFile(file string) ([]Annotation, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	filename := filepath.Base(file)

	var all []Annotation
	for lineNumber, line := range strings.Split(string(data), "\n") {
		p := strings.Index(line, "//@")
		if p < 0 {
			continue
		}

		all = append(all, Annotation{
			File:    filename,
			Line:    lineNumber + 1,
			Message: strings.TrimSpace(line[p+3:]),
		})
	}

	return all, nil
}

func build(t *testing.T) (exepath string, cleanup func()) {
	dir, err := ioutil.TempDir("", "unconvert_test")
	if err != nil {
		t.Fatalf("failed to create tempdir: %v\n", err)
	}
	exepath = filepath.Join(dir, "test_unconvert.exe")

	cleanup = func() {
		err := os.RemoveAll(dir)
		if err != nil {
			t.Fatal(err)
		}
	}

	output, err := exec.Command("go", "build", "-o", exepath, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build service program: %v\n%v", err, string(output))
	}

	return exepath, cleanup
}
