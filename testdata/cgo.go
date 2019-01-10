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
