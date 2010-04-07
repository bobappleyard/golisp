package main

import (
	"os"
	"github.com/bobappleyard/golisp/lisp"
)

func main() {
	//~ lisp.PreludePath = "./prelude.golisp"
	i := lisp.New()
	i.Repl(os.Stdin, os.Stdout)
}


