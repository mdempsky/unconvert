// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"sync"

	"golang.org/x/tools/container/intsets"
	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/types"
)

// Unnecessary conversions are identified as the offset of their left parenthesis within a source file.

func apply(file string, edits *intsets.Sparse) {
	if edits.IsEmpty() {
		return
	}

	var fset = token.NewFileSet()

	f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	v := editor{edits: edits, file: fset.File(f.Package)}
	ast.Walk(&v, f)

	// TODO(mdempsky): Write to temporary file and rename.
	var buf bytes.Buffer
	err = format.Node(&buf, fset, f)
	if err != nil {
		log.Fatal(err)
	}

	err = ioutil.WriteFile(file, buf.Bytes(), 0)
	if err != nil {
		log.Fatal(err)
	}
}

type editor struct {
	edits *intsets.Sparse
	file  *token.File
}

func (e *editor) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return nil
	}
	v := reflect.ValueOf(n).Elem()
	for i, n := 0, v.NumField(); i < n; i++ {
		switch f := v.Field(i).Addr().Interface().(type) {
		case *ast.Expr:
			e.rewrite(f)
		case *[]ast.Expr:
			for i := range *f {
				e.rewrite(&(*f)[i])
			}
		}
	}
	return e
}

func (e *editor) rewrite(f *ast.Expr) {
	if n, ok := (*f).(*ast.CallExpr); ok && e.edits.Has(e.file.Offset(n.Lparen)) {
		*f = n.Args[0]
	}
}

var (
	flagAll   = flag.Bool("all", false, "type check all GOOS and GOARCH combinations")
	flagApply = flag.Bool("apply", false, "apply edits")
)

func main() {
	flag.Parse()

	var m map[string]*intsets.Sparse
	if *flagAll {
		m = mergeEdits()
	} else {
		m = computeEdits(build.Default.GOOS, build.Default.GOARCH)
	}

	if *flagApply {
		var wg sync.WaitGroup
		for f, e := range m {
			wg.Add(1)
			f, e := f, e
			go func() {
				defer wg.Done()
				apply(f, e)
			}()
		}
		wg.Wait()
	} else {
		err := json.NewEncoder(os.Stdout).Encode(m)
		if err != nil {
			log.Fatal(err)
		}
		return
	}
}

var plats = [...]struct {
	goos, goarch string
}{
	{"linux", "386"},
	{"linux", "amd64"},
	{"linux", "arm"},
	{"linux", "arm64"},
	{"linux", "ppc64"},
	{"linux", "ppc64le"},
	{"nacl", "386"},
	{"nacl", "amd64p32"},
	{"nacl", "arm"},
	{"darwin", "386"},
	{"darwin", "amd64"},
	{"dragonfly", "amd64"},
	{"freebsd", "386"},
	{"freebsd", "amd64"},
	{"freebsd", "arm"},
	{"netbsd", "386"},
	{"netbsd", "amd64"},
	{"netbsd", "arm"},
	{"openbsd", "386"},
	{"openbsd", "amd64"},
	{"openbsd", "arm"},
	{"plan9", "386"},
	{"plan9", "amd64"},
	{"solaris", "amd64"},
	{"windows", "386"},
	{"windows", "amd64"},
}

func mergeEdits() map[string]*intsets.Sparse {
	ch := make(chan map[string]*intsets.Sparse)
	var wg sync.WaitGroup
	for _, plat := range plats {
		wg.Add(1)
		go func(goos, goarch string) {
			defer wg.Done()
			ch <- computeEdits(goos, goarch)
		}(plat.goos, plat.goarch)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	m := make(map[string]*intsets.Sparse)
	for m1 := range ch {
		for f, e := range m1 {
			if e0, ok := m[f]; ok {
				e0.IntersectionWith(e)
			} else {
				m[f] = e
			}
		}
	}
	return m
}

func noImport(map[string]*types.Package, string) (*types.Package, error) {
	panic("go/loader said this wouldn't be called")
}

func computeEdits(os, arch string) map[string]*intsets.Sparse {
	ctxt := build.Default
	ctxt.GOOS = os
	ctxt.GOARCH = arch
	ctxt.CgoEnabled = false

	var conf loader.Config
	conf.Build = &ctxt
	conf.TypeChecker.Import = noImport
	for _, arg := range flag.Args() {
		conf.Import(arg)
	}
	prog, err := conf.Load()
	if err != nil {
		log.Fatal(err)
	}

	type res struct {
		file  string
		edits *intsets.Sparse
	}
	ch := make(chan res)
	var wg sync.WaitGroup
	for _, pkg := range prog.InitialPackages() {
		for _, file := range pkg.Files {
			wg.Add(1)
			go func(pkg *loader.PackageInfo, file *ast.File) {
				defer wg.Done()
				v := visitor{pkg: pkg, file: conf.Fset.File(file.Package)}
				ast.Walk(&v, file)
				ch <- res{v.file.Name(), &v.edits}
			}(pkg, file)
		}
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	m := make(map[string]*intsets.Sparse)
	for r := range ch {
		m[r.file] = r.edits
	}
	return m
}

type visitor struct {
	pkg   *loader.PackageInfo
	file  *token.File
	edits intsets.Sparse
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	if call, ok := node.(*ast.CallExpr); ok {
		v.unconvert(call)
	}
	return v
}

func (v *visitor) unconvert(call *ast.CallExpr) {
	// TODO(mdempsky): Handle useless multi-conversions.

	// Conversions have exactly one argument.
	if len(call.Args) != 1 || call.Ellipsis != token.NoPos {
		return
	}
	ft, ok := v.pkg.Types[call.Fun]
	if !ok {
		fmt.Println("Missing type for function")
		return
	}
	if !ft.IsType() {
		// Not a conversion.
		return
	}
	at, ok := v.pkg.Types[call.Args[0]]
	if !ok {
		fmt.Println("Missing type for argument")
	}
	if !types.Identical(ft.Type, at.Type) {
		// A real conversion.
		return
	}
	if at.Value != nil || hasUntypedValue(call.Args[0]) {
		// As a workaround for golang.org/issue/13061,
		// skip conversions that contain an untyped value.
		return
	}

	v.edits.Insert(v.file.Offset(call.Lparen))
}

func hasUntypedValue(n ast.Expr) bool {
	var v uvVisitor
	ast.Walk(&v, n)
	return v.found
}

type uvVisitor struct {
	found bool
}

func (v *uvVisitor) Visit(node ast.Node) ast.Visitor {
	// Short circuit.
	if v.found {
		return nil
	}

	switch node := node.(type) {
	case *ast.BinaryExpr:
		switch node.Op {
		case token.SHL, token.SHR, token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ:
			// Shifts yield an untyped value if their LHS is untyped.
			// Comparisons yield an untyped boolean value.
			v.found = true
		}
	case *ast.Ident:
		if node.Name == "nil" {
			// Probably the universal untyped zero value.
			v.found = true
		}
	}

	if v.found {
		return nil
	}
	return v
}
