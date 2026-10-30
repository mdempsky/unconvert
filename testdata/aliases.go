// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testdata

// Conversions spelled through a type alias name may be intentional:
// the alias name can carry semantic meaning even though it denotes
// the same type (for example, int32 and rune). For #61.

type AliasInt = int
type AliasString = string

func Aliases() {
	var i int
	var s string
	var r rune
	var i32 int32
	var ai AliasInt
	var as AliasString

	// Not flagged: the names differ even though the types match.
	_ = AliasInt(i)
	_ = AliasString(s)
	_ = int(ai)
	_ = string(as)
	_ = rune(i32)
	_ = int32(r)

	// Still flagged: the names match on both sides.
	_ = AliasInt(ai)    //@ unnecessary conversion
	_ = AliasString(as) //@ unnecessary conversion
	_ = rune(r)         //@ unnecessary conversion
	_ = int(i)          //@ unnecessary conversion
}
