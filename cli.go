//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: merge out.pdf a.pdf b.pdf ...")
		os.Exit(2)
	}
	n, e := mergePDFFiles(os.Args[2:], os.Args[1])
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	fmt.Println("pages", n)
}
