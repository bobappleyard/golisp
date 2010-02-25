package lisp

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unsafe"
	"./errors"
)

/*
	Basic types
*/

type Any interface{}

type Symbol string

func (self Symbol) GoString() string {
	return string(self)
}

type Environment map[Symbol] Any

type Constant struct { 
	str string
}

func NewConstant(str string) Any {
	return &Constant { str }
}

func (self *Constant) String() string {
	return self.GoString()
}

func (self *Constant) GoString() string {
	return self.str
}

// Anything except the boolean value false counts as true.
func True(x Any) bool {
	if b, ok := x.(bool); ok {
		return b
	}
	return true
}

/*
	Functions
*/

type Function interface {
	Apply(args Any) Any
}

func Call(f Function, args... Any) Any {
	return f.Apply(vecToLs(Vector(args)))
}

type Primitive func(args Any) Any

func (self Primitive) Apply(args Any) Any {
	return self(args)
}

func (self Primitive) String() string {
	return self.GoString()
}

func (self Primitive) GoString() string {
	return "#<primitive>"
}

// Takes a function, which can take anything from none to five lisp.Any and 
// must return lisp.Any, and returns a function that can be called by the
// lisp system. Returns nil if it fails to match, which I suppose is pretty
// bad, really.
func WrapPrimitive(_f interface{}) Function {
	wrap := func(l int, f func(Vector) Any) Function {
		var res Function
		res = Primitive(func(args Any) Any {
			as, ok := lsToVec(args).(Vector)
			if !ok { return ArgumentError(res, args) }
			if len(as) != l { return ArgumentError(res, args) }
			return f(as)
		})
		return res
	}
	switch f := _f.(type) {
		case func() Any: return wrap(0, func(args Vector) Any { 
			return f() 
		})
		case func(a Any) Any: return wrap(1, func(args Vector) Any {
			return f(args[0])
		})
		case func(a, b Any) Any: return wrap(2, func(args Vector) Any { 
			return f(args[0], args[1])
		})
		case func(a, b, c Any) Any: return wrap(3, func(args Vector) Any { 
			return f(args[0], args[1], args[2])
		})
		case func(a, b, c, d Any) Any: return wrap(4, func(args Vector) Any { 
			return f(args[0], args[1], args[2], args[3])
		})
		case func(a, b, c, d, e  Any) Any: return wrap(5, func(args Vector) Any { 
			return f(args[0], args[1], args[2], args[3], args[4])
		})
	}
	return nil
}

// Takes a map, containing functions to be passed to WrapPrimitive. Returns 
// an environment.
func WrapPrimitives(env map[string] interface{}) Environment {
	res := make(Environment)
	for k, v := range env {
		f := WrapPrimitive(v)
		if f == nil { errors.Fatal(os.ErrorString("invalid primitive function: " + k)) }
		res[Symbol(k)] = f
	}
	return res
}

/*
	Errors
*/

type errorStruct struct {
	kind Symbol
	msg string
}

func (self *errorStruct) String() string {
	return self.GoString()
}

func (self *errorStruct) GoString() string {
	return fmt.Sprintf("%v: %s", self.kind, self.msg)
}

func Throw(kind Symbol, msg string) os.Error {
	return &errorStruct { kind, msg }
}

func Error(msg string) os.Error {
	return Throw(Symbol("error"), msg)
}

func TypeError(expected string, obj Any) os.Error {
	return Throw(Symbol("type-error"), fmt.Sprintf("expecting %s: %#v", expected, obj))
}

func ArgumentError(f, args Any) os.Error {
	msg := fmt.Sprintf("wrong number of arguments to %v: %#v", f, args)
	return Throw(Symbol("argument-error"), msg)
}

func SystemError(err os.Error) os.Error {
	return Throw(Symbol("system-error"), err.String())
}

func SyntaxError(err string) os.Error {
	return Throw(Symbol("syntax-error"), err)
}

func Failed(x Any) bool {
	_, failed := x.(*errorStruct)
	return failed
}

/*
	Pairs
*/

var EMPTY_LIST = NewConstant("()")

type Pair struct { a, d Any }

func (self *Pair) String() string {
	if self.d == EMPTY_LIST {
		return fmt.Sprintf("(%v)", self.a)
	}
	if d, ok := self.d.(*Pair); ok {
		return fmt.Sprintf("(%v %v", self.a, d.String()[1:])
	}
	return fmt.Sprintf("(%v . %v)", self.a, self.d)
}

func (self *Pair) GoString() string {
	if self.d == EMPTY_LIST {
		return fmt.Sprintf("(%#v)", self.a)
	}
	if d, ok := self.d.(*Pair); ok {
		return fmt.Sprintf("(%#v %v", self.a, d.GoString()[1:])
	}
	return fmt.Sprintf("(%#v . %#v)", self.a, self.d)
}

func Cons(a, d Any) Any {
	return &Pair { a, d }
}

func pairFunc(x Any, f func(*Pair) Any) Any {
	if Failed(x) { return x }
	p, ok := x.(*Pair)
	if !ok { return TypeError("pair", x) }
	return f(p)
}

func Car(x Any) Any {
	return pairFunc(x, func(p *Pair) Any { return p.a })
}

func Cdr(x Any) Any {
	return pairFunc(x, func(p *Pair) Any { return p.d })
}

func List(xs... Any) Any {
	return vecToLs(Vector(xs))
}

func ListLen(ls Any) int {
	res := 0
	for; ls != EMPTY_LIST; ls, res = Cdr(ls), res + 1 {
		if Failed(ls) { return -1 }
	}
	return res
}

func ListTail(ls Any, idx int) Any {
	for ; idx > 0; idx, ls = idx - 1, Cdr(ls) {
		if Failed(ls) { return ls }
	}
	return ls
}

func ListRef(ls Any, idx int) Any {
	return Car(ListTail(ls, idx))
}

/*
	Vectors
*/

type Vector []Any

func (self Vector) Get(i int) Any {
	if i < 0 || i >= len(self) { 
		return Error(fmt.Sprintf("invalid index (%v)", i))
	}
	return self[i]
}

func (self Vector) Set(i int, v Any) Any {
	if i < 0 || i >= len(self) { 
		return Error(fmt.Sprintf("invalid index (%v)", i))
	}
	self[i] = v
	return nil
}

func (self Vector) String() string {
	return self.GoString()
}

func (self Vector) GoString() string {
	res := "#("
	for _, x := range self {
		res += fmt.Sprintf("%v ", x)
	}
	return res[0:len(res)-1] + ")"
}

/*
	Custom types
*/

type Custom struct {
	name Symbol
	val Any
}

func NewCustom(name Symbol, val Any) *Custom {
	return &Custom { name, val }
}

func (self *Custom) String() string {
	return self.GoString()
}

func (self *Custom) GoString() string {
	_, addr := unsafe.Reflect(self)
	return fmt.Sprintf("#<%s: %x>", self.name, addr)
}

func (self *Custom) Name() Symbol {
	return self.name
}

func (self *Custom) Get() Any {
	return self.val
}

func (self *Custom) Set(val Any) {
	self.val = val
}

/*
	Evaluation related stuff
*/

type Context struct {
	env Environment
	parent *Context
}

type closure struct {
	ctx *Context
	vars, body Any
}

type macro struct {
	f Function
}

type tailStruct struct {
	f *Function
	args *Any
}

// Create a new execution context for some code.
func NewContext(parent *Context) *Context {
	return &Context { make(Environment), parent }
}

// Create a Context that can be used as an interpreter.
func New() *Context {
	res := NewContext(nil)
	res.Bind(Primitives())
	res.Bind(WrapPrimitives(map[string] interface{} {
		"root-environment": func() Any { return res },
	}))
	return res
}

// Contexts

func (self *Context) String() string {
	return self.GoString()
}

func (self *Context) GoString() string {
	return "#<environment>"
}

func (self *Context) Eval(x Any) Any {
	expr := self.Expand(x)
	return self.evalExpr(expr, nil)
}

func (self *Context) EvalString(x string) Any {
	return self.Eval(ReadString(x))
}

func (self *Context) Expand(x Any) Any {
	for {
		p, ok := x.(*Pair); if !ok { break }
		if s, ok := p.a.(Symbol); ok {
			switch string(s) {
				case "quote": break
				case "if": return Cons(p.a, self.expandList(p.d))
				case "lambda": return Cons(p.a, Cons(Car(p.d), self.expandList(Cdr(p.d))))
				case "set!": return List(p.a, Car(p.d), self.Expand(Car(Cdr(p.d))))
				case "define": return Cons(p.a, self.expandDefinition(p.d))
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

func (self *Context) Bind(env Environment) {
	for k, v := range env {
		self.env[k] = v
	}
}

func (self *Context) Lookup(x string) Any {
	return self.lookupSym(Symbol(x))
}

func (self *Context) Load(path string) os.Error {
	src, err := os.Open(path, os.O_RDONLY, 0)
	if err != nil { return SystemError(err) }
	for x := Read(src); x != EOF_OBJECT; x = Read(src) {
		r := self.Eval(x)
		if Failed(r) { return r.(os.Error) }		
	}
	return nil
}

func (self *Context) Repl(in io.Reader, out io.Writer) {
	read := func() Any {
		s, err := ReadLine(in)
		if err != nil { return err }
		if strings.TrimSpace(s) == "" { return nil }
		return ReadString(s)
	}
	Display("> ", out)
	for x := read(); x != EOF_OBJECT; x = read() {
		x = self.Eval(x)
		if x != nil {
			Write(x, out)
			Display("\n", out)
		}
		Display("> ", out)
	}
}



func (self *Context) evalExpr(_x Any, tail *tailStruct) Any {
	// pairs and symbols get treated specially
	switch x := _x.(type) {
		case *Pair: return self.evalPair(x, tail)
		case Symbol: return self.lookupSym(x)
	}
	// everything else is self-evaluating
	return _x
}

func (self *Context) evalPair(x *Pair, tail *tailStruct) Any {
	switch n := x.a.(type) {
		case Symbol: switch string(n) {
			// core forms
			case "quote": return Car(x.d)
			case "if": if True(self.evalExpr(ListRef(x.d, 0), nil)) {
				return self.evalExpr(ListRef(x.d, 1), tail)
			} else {
				return self.evalExpr(ListRef(x.d, 2), tail)
			}
			case "lambda": return &closure { self, Car(x.d), Cdr(x.d) }
			case "set!": return self.mutate(Car(x.d), Car(Cdr(x.d)))
			case "define": return self.evalDefine(x.d)
			case "begin": return self.evalBlock(x.d, tail)
			case "local-environment": return self
			// otherwise fall through to a function call
		}
		case *Pair: // do nothing, it's handled below
		default: return TypeError("pair or symbol", n)
	}
	// function application
	return self.evalCall(self.evalExpr(x.a, nil), x.d, tail)
}

func (self *Context) lookupSym(x Symbol) Any {
	if self == nil { return Error(fmt.Sprintf("unknown variable: %s", x)) }
	res, ok := self.env[x]
	if ok {
		return res
	}
	return self.parent.lookupSym(x)
}

func (self *Context) mutate(_name, val Any) Any {
	if self == nil {
		return Error(fmt.Sprintf("unknown variable: %s", _name))
	}
	name, ok := _name.(Symbol)
	if !ok {
		return TypeError("symbol", _name)
	}
	_, ok = self.env[name]
	if !ok {
		return self.parent.mutate(_name, val)
	}
	self.env[name] = val
	return nil
}

func (self *Context) evalCall(_f, args Any, tail *tailStruct) Any {
	if Failed(_f) { return _f }
	var argvals Any = EMPTY_LIST
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
	f, ok := _f.(Function)
	if !ok { return TypeError("function", _f) }
	// call it
	if tail == nil {
		return f.Apply(argvals)
	}
	// in tail position
	*(tail.f) = f
	*(tail.args) = argvals
	return nil
}

func (self *Context) evalDefine(ls Any) Any {
	d := Car(ls)
	if Failed(d) { return d }
	n, ok := d.(Symbol)
	if !ok { return TypeError("symbol", d) }
	d = Car(Cdr(ls))
	if Failed(d) { return d }
	self.env[n] = self.evalExpr(d, nil)
	return nil
}

func (self *Context) evalBlock(body Any, tail *tailStruct) Any {
	var res Any
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

func (self *Context) expandList(ls Any) Any {
	var res Any = EMPTY_LIST
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

func (self *Context) expandDefinition(ls Any) Any {
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

/* 
	Closures
*/

func (self *closure) String() string {
	return self.GoString()
}

func (self *closure) GoString() string {
	return fmt.Sprintf("#<closure %v>", self.vars)
}

func (self *closure) Apply(args Any) Any {
	var res Any
	var f Function = self
	// closures can tail recurse, the for loop captures this
	tail := &tailStruct { &f, &args }
	for f != nil {
		if cl, ok := f.(*closure); ok {
			f = nil
			ctx := NewContext(cl.ctx)
			err := cl.bindArgs(args)
			if err != nil { return err }
			res = ctx.evalBlock(cl.body, tail)
		} else {
			// primitive functions, or whatever
			return f.Apply(args)
		}
	}
	return res
}

func (self *closure) bindArgs(args Any) os.Error {
	vars := self.vars
	for {
		if Failed(args) { return args.(os.Error) }
		if vars == EMPTY_LIST && args == EMPTY_LIST {
			return nil
		}
		if vars == EMPTY_LIST {
			return ArgumentError(self, args)
		}
		p, pair := vars.(*Pair)
		if args == EMPTY_LIST && pair {
			return ArgumentError(self, args)
		}
		if !pair {
			return self.bindArg(vars, args)
		}
		err := self.bindArg(p.a, Car(args))
		if err != nil {
			return err
		}
		vars, args = p.d, Cdr(args)
	}
	panic("unreachable")
}

func (self *closure) bindArg(name, val Any) os.Error {
	n, ok := name.(Symbol)
	if !ok { return TypeError("symbol", name) }
	self.ctx.env[n] = val
	return nil
}

/*
	Macros
*/

func (self *macro) String() string {
	return self.GoString()
}

func (self *macro) GoString() string {
	return "#<macro>"
}

