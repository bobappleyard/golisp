package lisp

import (
	v "container/vector";
)

type Interpreter struct {
	// these are all (relatively) self-contained units of the interpreter
	environment;
	syntax;
	compile;
	exec;
}

type Initializer func(*Interpreter)

var initializers *v.Vector;

func RegisterInitializer(i Initializer) {
	if initializers == nil {
		initializers = new(v.Vector);
	}
	initializers.Push(i);
}

func NewInterpreter() *Interpreter {
	// general initialisation
	res := new(Interpreter);
	res.init();
	// externally defined initialisation
	for i := range initializers.Iter() {
		i.(Initializer)(res);
	}
	return res;
}

func (self *Interpreter) init() {
	self.initEnvironment();
	self.initSyntax(self);
	syms := self.initSyms();
	self.initCompile(syms);
	self.initExec(syms);
	self.initPrimitives();
}

func (self *Interpreter) REPL(in <-chan string, out chan<- string) {
	for e := range in {
		v := self.EvalString(e);
		out <- self.ToWrite(v);
	}
	close(out);
}