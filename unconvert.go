// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Unconvert removes redundant type conversions from Go packages.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"runtime/pprof"
	"sort"
	"sync"
	"unicode"

	"golang.org/x/tools/container/intsets"
	"golang.org/x/tools/go/loader"
)

// Unnecessary conversions are identified by the position
// of their left parenthesis within a source file.

func apply(file string, edits *intsets.Sparse) {
	if edits.IsEmpty() {
		return
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		log.Fatal(err)
	}

	// Note: We modify edits during the walk.
	v := editor{edits: edits, file: fset.File(f.Package)}
	ast.Walk(&v, f)
	if !edits.IsEmpty() {
		log.Printf("%s: missing edits %s", file, edits)
	}

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
	n, ok := (*f).(*ast.CallExpr)
	if !ok {
		return
	}
	off := e.file.Offset(n.Lparen)
	if !e.edits.Has(off) {
		return
	}
	*f = n.Args[0]
	e.edits.Remove(off)
}

func print(name string, edits *intsets.Sparse) {
	if edits.IsEmpty() {
		return
	}

	buf, err := ioutil.ReadFile(name)
	if err != nil {
		log.Fatal(err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, buf, 0)
	if err != nil {
		log.Fatal(err)
	}

	file := fset.File(f.Package)
	for _, p := range edits.AppendTo(nil) {
		pos := file.Position(file.Pos(p))
		if flagOneLiners {
			fmt.Printf("%s:%d:%d: useless conversion\n", pos.Filename, pos.Line,
				pos.Column)
		} else {
			fmt.Printf("%s:%d:%d:\n", pos.Filename, pos.Line, pos.Column)
			line := lineForOffset(buf, pos.Offset)
			fmt.Printf("%s\n", line)
			fmt.Printf("%s^\n", rub(line[:pos.Column-1]))
		}
	}
}

func rub(buf []byte) []byte {
	// TODO(mdempsky): Handle combining characters?
	// TODO(mdempsky): Handle East Asian wide characters?
	var res bytes.Buffer
	for _, c := range string(buf) {
		if !unicode.IsSpace(c) {
			c = ' '
		}
		res.WriteRune(c)
	}
	return res.Bytes()
}

func lineForOffset(buf []byte, off int) []byte {
	sol := bytes.LastIndexByte(buf[:off], '\n')
	if sol < 0 {
		sol = 0
	} else {
		sol += 1
	}
	eol := bytes.IndexByte(buf[off:], '\n')
	if eol < 0 {
		eol = len(buf)
	} else {
		eol += off
	}
	return buf[sol:eol]
}

var (
	flagAll        = flag.Bool("all", false, "type check all GOOS and GOARCH combinations")
	flagApply      = flag.Bool("apply", false, "apply edits to source files")
	flagCPUProfile = flag.String("cpuprofile", "", "write CPU profile to file")
	flagOneLiners = flag.Bool("oneliners", false, "outputs 1 line per case")
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: unconvert [flags] [package ...]\n")
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if *flagCPUProfile != "" {
		f, err := os.Create(*flagCPUProfile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	var m map[string]*intsets.Sparse
	if *flagAll {
		m = mergeEdits()
	} else {
		m = computeEdits(build.Default.GOOS, build.Default.GOARCH, build.Default.CgoEnabled)
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
		var files []string
		for f := range m {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, f := range files {
			print(f, m[f])
		}
	}
}

var plats = [...]struct {
	goos, goarch string
}{
	// TODO(mdempsky): buildall.bash also builds linux-386-387 and linux-arm-arm5.
	{"linux", "386"},
	{"linux", "amd64"},
	{"linux", "arm"},
	{"linux", "arm64"},
	{"linux", "mips64"},
	{"linux", "mips64le"},
	{"linux", "ppc64"},
	{"linux", "ppc64le"},
	{"nacl", "386"},
	{"nacl", "amd64p32"},
	{"nacl", "arm"},
	{"android", "386"},
	{"android", "amd64"},
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
	m := make(map[string]*intsets.Sparse)
	for _, plat := range plats {
		for f, e := range computeEdits(plat.goos, plat.goarch, false) {
			if e0, ok := m[f]; ok {
				e0.IntersectionWith(e)
			} else {
				m[f] = e
			}
		}
	}
	return m
}

type noImporter struct{}

func (noImporter) Import(path string) (*types.Package, error) {
	panic("golang.org/x/tools/go/loader said this wouldn't be called")
}

func computeEdits(os, arch string, cgoEnabled bool) map[string]*intsets.Sparse {
	ctxt := build.Default
	ctxt.GOOS = os
	ctxt.GOARCH = arch
	ctxt.CgoEnabled = cgoEnabled

	var conf loader.Config
	conf.Build = &ctxt
	conf.TypeChecker.Importer = noImporter{}
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
			pkg, file := pkg, file
			wg.Add(1)
			go func() {
				defer wg.Done()
				v := visitor{pkg: pkg, file: conf.Fset.File(file.Package)}
				ast.Walk(&v, file)
				ch <- res{v.file.Name(), &v.edits}
			}()
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
		// Function call; not a conversion.
		return
	}
	at, ok := v.pkg.Types[call.Args[0]]
	if !ok {
		fmt.Println("Missing type for argument")
	}
	if isUntypedValue(call.Args[0], &v.pkg.Info) {
		// Workaround golang.org/issue/13061.
		return
	}
	if !types.Identical(ft.Type, at.Type) {
		// A real conversion.
		return
	}

	v.edits.Insert(v.file.Offset(call.Lparen))
}

func isUntypedValue(n ast.Expr, info *types.Info) (res bool) {
	switch n := n.(type) {
	case *ast.BinaryExpr:
		switch n.Op {
		case token.SHL, token.SHR:
			// Shifts yield an untyped value if their LHS is untyped.
			return isUntypedValue(n.X, info)
		case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ:
			// Comparisons yield an untyped boolean value.
			return true
		case token.ADD, token.SUB, token.MUL, token.QUO, token.REM,
			token.AND, token.OR, token.XOR, token.AND_NOT,
			token.LAND, token.LOR:
			return isUntypedValue(n.X, info) && isUntypedValue(n.Y, info)
		}
	case *ast.UnaryExpr:
		switch n.Op {
		case token.ADD, token.SUB, token.NOT, token.XOR:
			return isUntypedValue(n.X, info)
		}
	case *ast.BasicLit:
		// Basic literals are always untyped.
		return true
	case *ast.ParenExpr:
		return isUntypedValue(n.X, info)
	case *ast.SelectorExpr:
		return isUntypedValue(n.Sel, info)
	case *ast.Ident:
		if obj, ok := info.Uses[n]; ok {
			if obj.Pkg() == nil && obj.Name() == "nil" {
				// The universal untyped zero value.
				return true
			}
			if b, ok := obj.Type().(*types.Basic); ok && b.Info()&types.IsUntyped != 0 {
				// Reference to an untyped constant.
				return true
			}
		}
	case *ast.CallExpr:
		if b, ok := asBuiltin(n.Fun, info); ok {
			switch b.Name() {
			case "real", "imag":
				return isUntypedValue(n.Args[0], info)
			case "complex":
				return isUntypedValue(n.Args[0], info) && isUntypedValue(n.Args[1], info)
			}
		}
	}

	return false
}

func asBuiltin(n ast.Expr, info *types.Info) (*types.Builtin, bool) {
	for {
		paren, ok := n.(*ast.ParenExpr)
		if !ok {
			break
		}
		n = paren.X
	}

	ident, ok := n.(*ast.Ident)
	if !ok {
		return nil, false
	}

	obj, ok := info.Uses[ident]
	if !ok {
		return nil, false
	}

	b, ok := obj.(*types.Builtin)
	return b, ok
}
