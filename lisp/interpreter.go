package lisp

import (
	"fmt"
	"io"
	"os"
	"strings"
)

/*
	Interpreter related stuff
*/

var PreludePath = ""

type Scope struct {
	env Environment
	parent *Scope
}

type closure struct {
	ctx *Scope
	vars, body interface{}
}

type macro struct {
	f Function
}

type tailStruct struct {
	f *Function
	args *interface{}
}

// Create a new execution Scope for some code.
func NewScope(parent *Scope) *Scope {
	return &Scope { make(Environment), parent }
}

// Create a Scope that can be used as an interpreter.
func New() (*Scope, os.Error) {
	res := NewScope(nil)
	res.Bind(Primitives())
	res.Bind(WrapPrimitives(map[string] interface{} {
		"root-environment": func() interface{} { return res },
	}))
	if PreludePath != "" {
		err := res.Load(PreludePath)
		if err != nil { return nil, err }
	}
	return res, nil
}

// Scopes

func (self *Scope) String() string {
	return self.GoString()
}

func (self *Scope) GoString() string {
	return "#<environment>"
}

func (self *Scope) Eval(x interface{}) interface{} {
	return self.evalExpr(self.Expand(x), nil)
}

func (self *Scope) EvalString(x string) interface{} {
	return self.Eval(ReadString(x))
}

func (self *Scope) Expand(x interface{}) interface{} {
	for {
		p, ok := x.(*Pair); if !ok { break }
		if s, ok := p.a.(Symbol); ok {
			switch string(s) {
				case "quote": return x
				case "if": return Cons(p.a, self.expandList(p.d))
				case "lambda": {
					ctx := NewScope(self)
					return Cons(p.a, Cons(Car(p.d), ctx.expandList(Cdr(p.d))))
				}
				case "set!": return List(p.a, Car(p.d), self.Expand(Car(Cdr(p.d))))
				case "define": return Cons(p.a, self.expandDefinition(p.d))
				case "define-macro": {
					expr := self.expandDefinition(p.d)
					expr = List(Symbol("define"), Car(expr), Cons(Symbol("macro"), Cdr(expr)))
					self.evalExpr(expr, nil)
					return expr
				}
				case "begin": return Cons(p.a, self.expandList(p.d))
			}
			if e := self.lookupSym(s); !Failed(e) {
				if m, ok := e.(*macro); ok {
					x = m.f.Apply(p.d)
					continue
				}
			}
		}
		x = self.expandList(x)
		break
	}
	return x
}

func (self *Scope) Bind(env Environment) {
	for k, v := range env {
		self.env[k] = v
	}
}

func (self *Scope) Lookup(x string) interface{} {
	return self.lookupSym(Symbol(x))
}

func (self *Scope) Load(path string) os.Error {
	_src, err := os.Open(path, os.O_RDONLY, 0)
	if err != nil { return SystemError(err) }
	src := NewInput(_src)
	exprs := ReadFile(src)
	if Failed(exprs) { return exprs.(os.Error) }
	for cur := exprs; cur != EMPTY_LIST; cur = Cdr(cur) {
		r := self.Eval(Car(cur))
		if Failed(r) { return r.(os.Error) }
	}
	return nil
}

func (self *Scope) Repl(in io.Reader, out io.Writer) {
	inp := NewInput(in)
	in = inp
	read := func() interface{} {
		s, err := inp.ReadLine()
		if err != nil { return err }
		if strings.TrimSpace(s) == "" { return nil }
		res := ReadString(s)
		if res == EOF_OBJECT { return nil }
		return res
	}
	self.Bind(WrapPrimitives(map[string] interface{} {
		"standard-input": func() interface{} { return in },
		"standard-output": func() interface{} { return out },
	}))
	Display("> ", out)
	for x := read(); !inp.Eof(); x = read() {
		x = self.Eval(x)
		if x != nil {
			Write(x, out)
			Display("\n", out)
		}
		Display("> ", out)
	}
	Display("\n", out)
}



func (self *Scope) evalExpr(_x interface{}, tail *tailStruct) interface{} {
	// pairs and symbols get treated specially
	switch x := _x.(type) {
		case *Pair: return self.evalPair(x, tail)
		case Symbol: return self.lookupSym(x)
	}
	// everything else is self-evaluating
	return _x
}

func (self *Scope) evalPair(x *Pair, tail *tailStruct) interface{} {
	switch n := x.a.(type) {
		case Symbol: switch string(n) {
			// standard forms
			case "quote": return Car(x.d)
			case "if": return self.evalIf(x.d, tail)
			case "lambda": return &closure { self, Car(x.d), Cdr(x.d) }
			case "set!": {
				v := self.evalExpr(Car(Cdr(x.d)), nil)
				if Failed(v) { return v }
				return self.mutate(Car(x.d), v)
			}
			case "define": return self.evalDefine(x.d)
			case "begin": return self.evalBlock(x.d, tail)
			// otherwise fall through to a function call
		}
		case *Pair: // do nothing, it's handled below
		default: return TypeError("pair or symbol", n)
	}
	// function application
	return self.evalCall(self.evalExpr(x.a, nil), x.d, tail)
}

func (self *Scope) lookupSym(x Symbol) interface{} {
	if self == nil { return Error(fmt.Sprintf("unknown variable: %s", x)) }
	res, ok := self.env[x]
	if ok {
		return res
	}
	return self.parent.lookupSym(x)
}

func (self *Scope) evalIf(expr interface{}, tail *tailStruct) interface{} {
	test := self.evalExpr(ListRef(expr, 0), nil)
	if Failed(test) { return test }
	if True(test) { return self.evalExpr(ListRef(expr, 1), tail) }
	return self.evalExpr(ListRef(expr, 2), tail)
}

func (self *Scope) mutate(_name, val interface{}) interface{} {
	if self == nil {
		return Error(fmt.Sprintf("unknown variable: %s", _name))
	}
	name, ok := _name.(Symbol)
	if !ok { return TypeError("symbol", _name) }
	_, ok = self.env[name]
	if !ok { return self.parent.mutate(_name, val) }
	self.env[name] = val
	return nil
}

func (self *Scope) evalCall(f, args interface{}, tail *tailStruct) interface{} {
	if Failed(f) { return f }
	var argvals interface{} = EMPTY_LIST
	p := new(Pair)
	// evaluate the arguments
	for cur := args; cur != EMPTY_LIST; cur = Cdr(cur) {
		if argvals == EMPTY_LIST {
			argvals = p
		}
		r := self.evalExpr(Car(cur), nil)
		if Failed(r) { return r }
		p.a = r
		if Cdr(cur) == EMPTY_LIST {
			p.d = EMPTY_LIST
			break
		}
		next := new(Pair)
		p.d = next
		p = next
	}
	// get the function
	fn, ok := f.(Function)
	if !ok { return TypeError("function", f) }
	// call it
	if tail == nil {
		return fn.Apply(argvals)
	}
	// in tail position
	*(tail.f) = fn
	*(tail.args) = argvals
	return nil
}

func (self *Scope) evalDefine(ls interface{}) interface{} {
	d := Car(ls)
	if Failed(d) { return d }
	n, ok := d.(Symbol)
	if !ok { return TypeError("symbol", d) }
	d = Car(Cdr(ls))
	if Failed(d) { return d }
	d = self.evalExpr(d, nil)
	if Failed(d) { return d }
	self.env[n] = d
	return nil
}

func (self *Scope) evalBlock(body interface{}, tail *tailStruct) interface{} {
	var res interface{}
	for cur := body; cur != EMPTY_LIST; cur = Cdr(cur) {
		if Cdr(cur) == EMPTY_LIST { // in tail position
			res = self.evalExpr(Car(cur), tail)
		} else {
			v := self.evalExpr(Car(cur), nil)
			if Failed(v) { return v }
		}
	}
	return res
}

func (self *Scope) expandList(ls interface{}) interface{} {
	var res interface{} = EMPTY_LIST
	p := new(Pair)
	for cur := ls; cur != EMPTY_LIST; cur = Cdr(cur) {
		if res == EMPTY_LIST {
			res = p
		}
		p.a = self.Expand(Car(cur))
		next := new(Pair)
		if Cdr(cur) == EMPTY_LIST {
			p.d = EMPTY_LIST
			break
		}
		if _, ok := Cdr(cur).(*Pair); !ok {
			p.d = self.Expand(Cdr(cur))
			break
		}
		p.d = next
		p = next
	}
	return res
}

func (self *Scope) expandDefinition(ls interface{}) interface{} {
	for {
		if p, ok := Car(ls).(*Pair); ok {
			ls = List(p.a, Cons(Symbol("lambda"), Cons(p.d, Cdr(ls))))
		} else {
			ls = Cons(Car(ls), self.expandList(Cdr(ls)))
			break
		}
	}
	return ls
}

// Closures

func (self *closure) String() string {
	return self.GoString()
}

func (self *closure) GoString() string {
	return fmt.Sprintf("#<closure %v>", self.vars)
}

func (self *closure) Apply(args interface{}) interface{} {
	var res interface{}
	var f Function = self
	// closures can tail recurse, the for loop captures this
	tail := &tailStruct { &f, &args }
	for f != nil {
		if cl, ok := f.(*closure); ok {
			f = nil
			ctx := NewScope(cl.ctx)
			err := cl.bindArgs(ctx, args)
			if err != nil { return err }
			res = ctx.evalBlock(cl.body, tail)
		} else {
			// primitive functions, or whatever
			return f.Apply(args)
		}
	}
	return res
}

func (self *closure) bindArgs(ctx *Scope, args interface{}) os.Error {
	vars := self.vars
	for {
		if Failed(args) { return args.(os.Error) }
		if vars == EMPTY_LIST && args == EMPTY_LIST { return nil }
		if vars == EMPTY_LIST { return ArgumentError(self, args) }
		p, pair := vars.(*Pair)
		if args == EMPTY_LIST && pair { return ArgumentError(self, args) }
		if !pair { return self.bindArg(ctx, vars, args) }
		err := self.bindArg(ctx, p.a, Car(args))
		if err != nil { return err }
		vars, args = p.d, Cdr(args)
	}
	panic("unreachable")
}

func (self *closure) bindArg(ctx *Scope, name, val interface{}) os.Error {
	n, ok := name.(Symbol)
	if !ok { return TypeError("symbol", name) }
	ctx.env[n] = val
	return nil
}

// Macros

func (self *macro) String() string {
	return self.GoString()
}

func (self *macro) GoString() string {
	return "#<macro>"
}

