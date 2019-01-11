// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdata

// void foo(int x) {}
// void bar(int* x) {}
import "C"

// Basic validity tests for C calls.
func _() {
	C.foo(0)
	C.foo(C.int(0))
	C.foo(C.int(C.int(0))) //@ unnecessary conversion

	C.bar(nil)
	C.bar((*C.int)(nil))
	C.bar((*C.int)((*C.int)(nil))) //@ unnecessary conversion
}

// Issue #39: don't warn about cgo-generated files.
func _() interface{} {
	return C.foo
}
