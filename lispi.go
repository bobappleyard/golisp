package main

import (
	"./lisp";
	"os";
)

func main() {
	i := lisp.New()
	i.Repl(os.Stdin, os.Stdout)
}


