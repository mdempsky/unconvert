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
	"strconv"
	"strings"
	"testing"
)

func TestBinary(t *testing.T) {
	exePath, cleanup := build(t)
	defer cleanup()

	output, err := exec.Command(exePath, "./testdata").CombinedOutput()
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

	need := map[Annotation]struct{}{}
	for _, annotation := range got {
		need[annotation] = struct{}{}
	}

	for _, annotation := range expected {
		_, ok := need[annotation]
		if ok {
			delete(need, annotation)
		} else {
			t.Errorf("unexpected: %v", annotation)
		}
	}

	for annotation := range need {
		t.Errorf("missing: %v", annotation)
	}
}

type Annotation struct {
	File    string
	Line    int
	Message string
}

func (ann Annotation) String() string {
	return fmt.Sprintf("%s:%d: %s", ann.File, ann.Line, ann.Message)
}

func ParseOutput(dir, output string) ([]Annotation, error) {
	var all []Annotation
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		folderStart := strings.Index(line, dir)
		if folderStart < 0 {
			continue
		}

		line = line[folderStart+len(dir)+1:]
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
		if filepath.Ext(file.Name()) != ".go" {
			continue
		}

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

func build(t *testing.T) (exePath string, cleanup func()) {
	dir, err := ioutil.TempDir("", "unconvert_test")
	if err != nil {
		t.Fatalf("failed to create tempdir: %v\n", err)
	}
	exePath = filepath.Join(dir, "test_unconvert.exe")

	cleanup = func() {
		err := os.RemoveAll(dir)
		if err != nil {
			t.Fatal(err)
		}
	}

	output, err := exec.Command("go", "build", "-o", exePath, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build service program: %v\n%v", err, string(output))
	}

	return exePath, cleanup
}
