// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdata

import "io"

// Basic contains conversion errors for builtin data types
func Basic() {
	var vbool bool
	var vbyte byte
	var vcomplex128 complex128
	var vcomplex64 complex64
	var verror error
	var vfloat32 float32
	var vfloat64 float64
	var vint int
	var vint16 int16
	var vint32 int32
	var vint64 int64
	var vint8 int8
	var vrune rune
	var vstring string
	var vuint uint
	var vuint16 uint16
	var vuint32 uint32
	var vuint64 uint64
	var vuint8 uint8
	var vuintptr uintptr

	_ = bool(vbool)       //@ unnecessary conversion
	_ = byte(vbyte)       //@ unnecessary conversion
	_ = error(verror)     //@ unnecessary conversion
	_ = int(vint)         //@ unnecessary conversion
	_ = int16(vint16)     //@ unnecessary conversion
	_ = int32(vint32)     //@ unnecessary conversion
	_ = int64(vint64)     //@ unnecessary conversion
	_ = int8(vint8)       //@ unnecessary conversion
	_ = rune(vrune)       //@ unnecessary conversion
	_ = string(vstring)   //@ unnecessary conversion
	_ = uint(vuint)       //@ unnecessary conversion
	_ = uint16(vuint16)   //@ unnecessary conversion
	_ = uint32(vuint32)   //@ unnecessary conversion
	_ = uint64(vuint64)   //@ unnecessary conversion
	_ = uint8(vuint8)     //@ unnecessary conversion
	_ = uintptr(vuintptr) //@ unnecessary conversion

	_ = float32(vfloat32)
	_ = float64(vfloat64)
	_ = complex128(vcomplex128)
	_ = complex64(vcomplex64)

	// Pointers
	_ = (*bool)(&vbool)             //@ unnecessary conversion
	_ = (*byte)(&vbyte)             //@ unnecessary conversion
	_ = (*complex128)(&vcomplex128) //@ unnecessary conversion
	_ = (*complex64)(&vcomplex64)   //@ unnecessary conversion
	_ = (*error)(&verror)           //@ unnecessary conversion
	_ = (*float32)(&vfloat32)       //@ unnecessary conversion
	_ = (*float64)(&vfloat64)       //@ unnecessary conversion
	_ = (*int)(&vint)               //@ unnecessary conversion
	_ = (*int16)(&vint16)           //@ unnecessary conversion
	_ = (*int32)(&vint32)           //@ unnecessary conversion
	_ = (*int64)(&vint64)           //@ unnecessary conversion
	_ = (*int8)(&vint8)             //@ unnecessary conversion
	_ = (*rune)(&vrune)             //@ unnecessary conversion
	_ = (*string)(&vstring)         //@ unnecessary conversion
	_ = (*uint)(&vuint)             //@ unnecessary conversion
	_ = (*uint16)(&vuint16)         //@ unnecessary conversion
	_ = (*uint32)(&vuint32)         //@ unnecessary conversion
	_ = (*uint64)(&vuint64)         //@ unnecessary conversion
	_ = (*uint8)(&vuint8)           //@ unnecessary conversion
	_ = (*uintptr)(&vuintptr)       //@ unnecessary conversion
}

// Counter is an int64
type Counter int64

// ID is a typed identifier
type ID string

// Metric is a struct
type Metric struct {
	ID      ID
	Counter Counter
}

// Custom contains conversion errors for builtin data types
func Custom() {
	type Local struct{ id ID }

	var counter Counter
	var id ID
	var m Metric
	var local Local
	var x struct{ id ID }

	_ = Counter(counter)     //@ unnecessary conversion
	_ = ID(id)               //@ unnecessary conversion
	_ = Metric(m)            //@ unnecessary conversion
	_ = Local(local)         //@ unnecessary conversion
	_ = (struct{ id ID })(x) //@ unnecessary conversion

	// Pointers
	_ = (*Counter)(&counter)   //@ unnecessary conversion
	_ = (*ID)(&id)             //@ unnecessary conversion
	_ = (*Metric)(&m)          //@ unnecessary conversion
	_ = (*Local)(&local)       //@ unnecessary conversion
	_ = (*struct{ id ID })(&x) //@ unnecessary conversion
}

// Interfaces contains conversion errors for interfaces
func Interfaces() {
	var writer io.Writer

	_ = (io.Writer)(writer)   //@ unnecessary conversion
	_ = (*io.Writer)(&writer) //@ unnecessary conversion
}

// Constructor is a func type
type Constructor func() ID

// Funcs contains conversion errors for func types
func Funcs() {
	type Local func(ID)
	type Recursive func(Recursive)

	var ctor Constructor
	var local Local
	var recursive Recursive

	_ = Constructor(ctor)    //@ unnecessary conversion
	_ = Local(local)         //@ unnecessary conversion
	_ = Recursive(recursive) //@ unnecessary conversion

	_ = (*Constructor)(&ctor)    //@ unnecessary conversion
	_ = (*Local)(&local)         //@ unnecessary conversion
	_ = (*Recursive)(&recursive) //@ unnecessary conversion
}
