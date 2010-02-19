GoLisp -- A dialect of Lisp written in Go
=========================================

Hello everyone! Here's a very, very primitive Lisp implementation in Go.

Compiling GoLisp
----------------

1. Download [Gobuild](http://code.google.com/p/gobuild/), compile it, and place the resulting
binary in your `$PATH`.

2. Download [bwl](http://github.com/bobappleyard/bwl) and place links to the directories named
`errors`, `lexer`, and `peg` in the directory this README resides in.

3. Open a terminal, navigate to the directory this README is in, and then run `gobuild` without
any parameters.


You should now have a program named "lispi" in the current directory.

    $ ./lispi

Gives you a REPL.