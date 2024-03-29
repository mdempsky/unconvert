# About

The unconvert program analyzes Go packages to identify unnecessary
type conversions; i.e., expressions T(x) where x already has type T.

# Install

    go install github.com/mdempsky/unconvert@latest

# Usage

    $ unconvert -v bytes fmt
    GOROOT/src/bytes/reader.go:117:14: unnecessary conversion
                    abs = int64(r.i) + offset
                               ^
    GOROOT/src/fmt/print.go:411:21: unnecessary conversion
            p.fmt.integer(int64(v), 16, unsigned, udigits)
                               ^

# Flags

Using the -v flag, unconvert will also print the source line and a
caret to indicate the unnecessary conversion's position therein.

Using the -apply flag, unconvert will rewrite the Go source files
without the unnecessary type conversions.

Using the -all flag, unconvert will analyze the Go packages under all
possible GOOS/GOARCH combinations, and only identify conversions that
are unnecessary in all cases.

E.g., syscall.Timespec's Sec and Nsec fields are int64 under
linux/amd64 but int32 under linux/386.  An int64(ts.Sec) conversion
that appears in a linux/amd64-only file will be identified as
unnecessary, but it will be preserved if it occurs in a file that's
compiled for both linux/amd64 and linux/386.
