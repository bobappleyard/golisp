package main

import (
	"github.com/bobappleyard/golisp/lisp"
	"os"
)

func main() {
	i := lisp.New()
	i.Repl(os.Stdin, os.Stdout)
}
