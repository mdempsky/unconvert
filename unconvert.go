// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"sync"

	"golang.org/x/tools/go/loader"
	"golang.org/x/tools/go/types"
)

var fset = token.NewFileSet()

type Edit struct {
	Pos, End int
}

type editsByPos []Edit

func (e editsByPos) Len() int           { return len(e) }
func (e editsByPos) Less(i, j int) bool { return e[i].Pos < e[j].Pos }
func (e editsByPos) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }

func apply(file string, edits []Edit) {
	if len(edits) == 0 {
		return
	}

	sort.Sort(editsByPos(edits))

	// Check for overlap.
	// TODO(mdempsky): Overlap can legally happen in bizarro expressions like
	// "(*[unsafe.Sizeof(int8(int8(0)))]byte)(*[1]byte)(nil)".
	for i := 1; i < len(edits); i++ {
		if edits[i-1].End > edits[i].Pos {
			log.Fatal("overlap")
		}
	}

	buf, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	n := edits[0].Pos
	for i := 1; i < len(edits); i++ {
		n += copy(buf[n:], buf[edits[i-1].End:edits[i].Pos])
	}
	n += copy(buf[n:], buf[edits[len(edits)-1].End:])
	buf = buf[:n]

	buf, err = format.Source(buf)
	if err != nil {
		log.Fatal(err)
	}

	err = ioutil.WriteFile(file, buf, 0)
	if err != nil {
		log.Fatal(err)
	}
}

var (
	flagAll   = flag.Bool("all", false, "type check all GOOS and GOARCH combinations")
	flagApply = flag.Bool("apply", false, "apply edits")
	flagGob   = flag.Bool("gob", false, "dump edits to stdout as gob")
)

func main() {
	flag.Parse()

	var m map[string][]Edit
	if *flagAll {
		m = mergeEdits()
	} else {
		m = computeEdits()
	}

	if *flagGob {
		err := gob.NewEncoder(os.Stdout).Encode(m)
		if err != nil {
			log.Fatal(err)
		}
		return
	} else if *flagApply {
		for f, e := range m {
			apply(f, e)
		}
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

func mergeEdits() map[string][]Edit {
	ch := make(chan map[string][]Edit)
	var wg sync.WaitGroup
	for _, plat := range plats {
		wg.Add(1)
		go func(goos, goarch string) {
			defer wg.Done()
			ch <- doSub(goos, goarch)
		}(plat.goos, plat.goarch)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	m := make(map[string][]Edit)
	for m1 := range ch {
		for f, e := range m1 {
			if e0, ok := m[f]; ok {
				m[f] = intersect(e0, e)
			} else {
				m[f] = e
			}
		}
	}
	return m
}

func intersect(e1, e2 []Edit) []Edit {
	if len(e1) == 0 || len(e2) == 0 {
		return nil
	}

	set := make(map[Edit]bool, len(e1))
	for _, e := range e1 {
		set[e] = true
	}

	var res []Edit
	for _, e := range e2 {
		if set[e] {
			res = append(res, e)
		}
	}
	return res
}

func doSub(goos, goarch string) map[string][]Edit {
	var m map[string][]Edit
	pr, pw := io.Pipe()
	ch := make(chan error)
	go func() {
		ch <- gob.NewDecoder(pr).Decode(&m)
	}()
	cmd := exec.Command("./unconvert", append([]string{"-gob"}, flag.Args()...)...)
	cmd.Stdout = pw
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch)
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
	if err := <-ch; err != nil {
		log.Fatal(err)
	}
	return m
}

func computeEdits() map[string][]Edit {
	var conf loader.Config
	conf.Fset = fset
	for _, arg := range flag.Args() {
		conf.Import(arg)
	}
	prog, err := conf.Load()
	if err != nil {
		log.Fatal(err)
	}

	type res struct {
		file  string
		edits []Edit
	}
	ch := make(chan res)
	var wg sync.WaitGroup
	for _, pkg := range prog.InitialPackages() {
		for _, file := range pkg.Files {
			wg.Add(1)
			go func(pkg *loader.PackageInfo, file *ast.File) {
				defer wg.Done()
				v := visitor{pkg: pkg, file: fset.File(file.Package)}
				ast.Walk(&v, file)
				sort.Sort(editsByPos(v.edits))
				ch <- res{v.file.Name(), v.edits}
			}(pkg, file)
		}
	}
	go func() {
		wg.Wait()
		close(ch)
	}()

	m := make(map[string][]Edit)
	for r := range ch {
		m[r.file] = r.edits
	}
	return m
}

type visitor struct {
	pkg   *loader.PackageInfo
	file  *token.File
	edits []Edit
	nodes []ast.Node
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	if node != nil {
		v.nodes = append(v.nodes, node)
	} else {
		v.nodes = v.nodes[:len(v.nodes)-1]
	}
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

	outer := v.nodes[len(v.nodes)-2]
	if keepParen(outer, call, call.Args[0]) {
		v.remove(call.Fun.Pos(), call.Lparen)
	} else {
		v.remove(call.Fun.Pos(), call.Lparen+1)
		v.remove(call.Rparen, call.Rparen+1)
	}
}

func (v *visitor) remove(pos, end token.Pos) {
	v.edits = append(v.edits, Edit{v.file.Offset(pos), v.file.Offset(end)})
}

func keepParen(a ast.Node, b, c ast.Expr) bool {
	// 1. Find the value in a that points to b.
	bp := findExprField(a, b)

	// 2. Try printing a with s/b/c/ and with s/b/(c)/.
	var buf1 bytes.Buffer
	*bp = c
	printer.Fprint(&buf1, fset, a)

	var buf2 bytes.Buffer
	*bp = &ast.ParenExpr{X: c}
	printer.Fprint(&buf2, fset, a)

	*bp = b

	// 3. Return whether they print the same (i.e., the parentheses are necessary).
	return buf1.String() == buf2.String()
}

func findExprField(a ast.Node, b ast.Expr) *ast.Expr {
	v := reflect.ValueOf(a).Elem()
	for i, n := 0, v.NumField(); i < n; i++ {
		// Interesting fields are either ast.Expr or []ast.Expr.
		switch f := v.Field(i).Interface().(type) {
		case ast.Expr:
			if f == b {
				return v.Field(i).Addr().Interface().(*ast.Expr)
			}
		case []ast.Expr:
			for i, e := range f {
				if e == b {
					return &f[i]
				}
			}
		}
	}
	log.Fatal("Failed to find b in a")
	return nil
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
