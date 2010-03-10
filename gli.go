package main

import (
	"os"
	"./lisp"
	"./errors"
)

func main() {
	i, err := lisp.New()
	errors.Fatal(err)
	i.Repl(os.Stdin, os.Stdout)
}


