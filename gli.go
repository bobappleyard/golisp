package main

import (
	"os"
	"./lisp"
	"./errors"
)

func main() {
	lisp.PreludePath = "./prelude.golisp"
	i, err := lisp.New()
	errors.Fatal(err)
	i.Repl(os.Stdin, os.Stdout)
}


