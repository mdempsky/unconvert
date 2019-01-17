// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdata

// Various explicit conversions of untyped constants
// that cannot be removed.
func _() {
	const (
		_ = byte(0)
		_ = int((real)(0i))
		_ = complex64(complex(1, 2))
		_ = (bool)(true || false)

		PtrSize = 4 << (^uintptr(0) >> 63)
		c0      = uintptr(PtrSize)
		c1      = uintptr((8-PtrSize)/4*2860486313 + (PtrSize-4)/4*33054211828000289)
	)

	i := int64(0)
	_ = i
}

// Make sure we distinguish function calls from
// conversion to function type.
func _() {
	type F func(F) int
	var f F

	_ = F(F(nil)) //@ unnecessary conversion
	_ = f(F(nil))
}

// Make sure we don't remove explicit conversions that
// prevent fusing floating-point operation.
// TODO(mdempsky): Test -fastmath=true.
func _() {
	var f1, f2, f3, ftmp float64
	_ = f1 + float64(f2*f3)
	ftmp = float64(f2 * f3)
	_ = f1 + ftmp
	ftmp = f2 * f3
	_ = f1 + float64(ftmp)

	var c1, c2, c3, ctmp complex128
	_ = c1 + complex128(c2*c3)
	ctmp = complex128(c2 * c3)
	_ = c1 + ctmp
	ctmp = c2 * c3
	_ = c1 + complex128(ctmp)
}
