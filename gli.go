package main

import (
	"os"
	"github.com/bobappleyard/golisp/lisp"
)

func main() {
	i := lisp.New()
	i.Repl(os.Stdin, os.Stdout)
}


