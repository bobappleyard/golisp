package main

import (
	"os"
	"./lisp"
)

func main() {
	//~ lisp.PreludePath = "./prelude.golisp"
	i := lisp.New()
	i.Repl(os.Stdin, os.Stdout)
}


