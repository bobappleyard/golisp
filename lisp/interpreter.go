package lisp

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/bobappleyard/bwl/errors"
)

/*
	Interpreter related stuff
*/

var PreludeFile = "prelude.golisp"
var PreludePaths = []string{}

func init() {
	if wd, err := os.Getwd(); err == nil {
		PreludePaths = append(PreludePaths, filepath.Join(wd, PreludeFile))
	}
	ps := strings.Split(os.Getenv("GOPATH"), string(filepath.ListSeparator))
	for _, p := range ps {
		PreludePaths = append(PreludePaths, filepath.Join(p, "src/github.com/bobappleyard/golisp", PreludeFile))
	}
	if u, err := user.Current(); err == nil {
		f := filepath.Join(u.HomeDir, ".golisp", PreludeFile)
		PreludePaths = append(PreludePaths, f)
	}
}

type Scope struct {
	env    Environment
	parent *Scope
}

type closure struct {
	ctx        *Scope
	vars, body interface{}
}

type macro struct {
	f Function
}

type tailStruct struct {
	f    *Function
	args *interface{}
}

// Create a new execution Scope for some code.
func NewScope(parent *Scope) *Scope {
	return &Scope{make(Environment), parent}
}

// patchy workaround...
func tryLoad(paths []string) string {
	for _, v := range paths {
		// nope! openFile still panics "unconditionally"
		//		if !Failed(openFile(v, Symbol("read"))) {
		if _, err := os.Open(v); err == nil {
			return v
		}
	}
	return ""
}

// Create a Scope that can be used as an interpreter.
func New() *Scope {
	res := NewScope(nil)
	res.Bind(Primitives())
	res.Bind(WrapPrimitives(map[string]interface{}{
		"root-environment": func() interface{} { return res },
	}))
	if PreludePath := tryLoad(PreludePaths); PreludePath != "" {
		res.Load(PreludePath)
	} else {
		panic("could not find " + PreludeFile + " file")
	}
	return res
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
	done := false
	for !done {
		p, ok := x.(*Pair)
		if !ok {
			break
		}
		if s, ok := p.a.(Symbol); ok {
			switch string(s) {
			case "quote":
				return x
			case "if":
				return Cons(p.a, self.expandList(p.d))
			case "lambda":
				{
					ctx := NewScope(self)
					return Cons(p.a, Cons(Car(p.d), ctx.expandList(Cdr(p.d))))
				}
			case "set!":
				return List(p.a, Car(p.d), self.Expand(Car(Cdr(p.d))))
			case "define":
				return Cons(p.a, self.expandDefinition(p.d))
			case "define-macro":
				{
					expr := self.expandDefinition(p.d)
					expr = List(Symbol("define"), Car(expr), Cons(Symbol("macro"), Cdr(expr)))
					self.evalExpr(expr, nil)
					return expr
				}
			case "begin":
				return Cons(p.a, self.expandList(p.d))
			}
			errors.Catch(
				func() { x = self.lookupSym(s).(*macro).f.Apply(p.d) },
				func(_ interface{}) { x, done = self.expandList(x), true },
			)
		} else {
			x, done = self.expandList(x), true
		}
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

func (self *Scope) Load(path string) {
	src := openFile(path, Symbol("read"))
	exprs := ReadFile(src)
	for cur := exprs; cur != EMPTY_LIST; cur = Cdr(cur) {
		self.Eval(Car(cur))
	}
}

func (self *Scope) Repl(in io.Reader, out io.Writer) {
	// set stuff up
	inp := NewInput(in)
	outp := NewOutput(out)
	read := func() interface{} {
		res := inp.ReadLine()
		if res == EOF_OBJECT {
			return nil
		}
		s := res.(string)
		if strings.TrimSpace(s) == "" {
			return nil
		}
		res = ReadString(s)
		if res == EOF_OBJECT {
			return nil
		}
		return res
	}
	self.Bind(WrapPrimitives(map[string]interface{}{
		"standard-input":  func() interface{} { return inp },
		"standard-output": func() interface{} { return outp },
	}))
	// main loop
	var x interface{}
	for !inp.Eof() {
		errors.Catch(
			func() {
				Display("> ", outp)
				outp.Flush()
				x = self.Eval(read())
			},
			func(err interface{}) { x = err },
		)
		if x != nil {
			Write(x, outp)
			Display("\n", outp)
		}
	}
	Display("\n", outp)
	outp.Flush()
}

func (self *Scope) evalExpr(_x interface{}, tail *tailStruct) interface{} {
	// pairs and symbols get treated specially
	switch x := _x.(type) {
	case *Pair:
		return self.evalPair(x, tail)
	case Symbol:
		return self.lookupSym(x)
	}
	// everything else is self-evaluating
	return _x
}

func (self *Scope) evalPair(x *Pair, tail *tailStruct) interface{} {
	switch n := x.a.(type) {
	case Symbol:
		switch string(n) {
		// standard forms
		case "quote":
			return Car(x.d)
		case "if":
			if True(self.evalExpr(ListRef(x.d, 0), nil)) {
				return self.evalExpr(ListRef(x.d, 1), tail)
			} else {
				return self.evalExpr(ListRef(x.d, 2), tail)
			}
		case "lambda":
			return &closure{self, Car(x.d), Cdr(x.d)}
		case "set!":
			{
				v := self.evalExpr(ListRef(x.d, 1), nil)
				self.mutate(Car(x.d), v)
				return nil
			}
		case "define":
			{
				self.evalDefine(x.d)
				return nil
			}
		case "begin":
			return self.evalBlock(x.d, tail)
			// otherwise fall through to a function call
		}
	case *Pair: // do nothing, it's handled below
	default:
		TypeError("pair or symbol", n)
	}
	// function application
	return self.evalCall(self.evalExpr(x.a, nil), x.d, tail)
}

func (self *Scope) lookupSym(x Symbol) interface{} {
	if self == nil {
		Error(fmt.Sprintf("unknown variable: %s", x))
	}
	res, ok := self.env[x]
	if ok {
		return res
	}
	return self.parent.lookupSym(x)
}

func (self *Scope) mutate(_name, val interface{}) {
	if self == nil {
		Error(fmt.Sprintf("unknown variable: %s", _name))
	}
	name, ok := _name.(Symbol)
	if !ok {
		TypeError("symbol", _name)
	}
	_, ok = self.env[name]
	if !ok {
		self.parent.mutate(_name, val)
	}
	self.env[name] = val
}

func (self *Scope) evalCall(f, args interface{}, tail *tailStruct) interface{} {
	var argvals interface{} = EMPTY_LIST
	p := new(Pair)
	// evaluate the arguments
	for cur := args; cur != EMPTY_LIST; cur = Cdr(cur) {
		if argvals == EMPTY_LIST {
			argvals = p
		}
		r := self.evalExpr(Car(cur), nil)
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
	if !ok {
		TypeError("function", f)
	}
	// call it
	if tail == nil {
		return fn.Apply(argvals)
	}
	// in tail position
	*(tail.f) = fn
	*(tail.args) = argvals
	return nil
}

func (self *Scope) evalDefine(ls interface{}) {
	d := Car(ls)
	n, ok := d.(Symbol)
	if !ok {
		TypeError("symbol", d)
	}
	d = Car(Cdr(ls))
	d = self.evalExpr(d, nil)
	self.env[n] = d
}

func (self *Scope) evalBlock(body interface{}, tail *tailStruct) interface{} {
	var res interface{}
	for cur := body; cur != EMPTY_LIST; cur = Cdr(cur) {
		if Cdr(cur) == EMPTY_LIST { // in tail position
			res = self.evalExpr(Car(cur), tail)
		} else {
			self.evalExpr(Car(cur), nil)
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
	tail := &tailStruct{&f, &args}
	for f != nil {
		if cl, ok := f.(*closure); ok {
			f = nil
			ctx := NewScope(cl.ctx)
			cl.bindArgs(ctx, args)
			res = ctx.evalBlock(cl.body, tail)
		} else {
			// primitive functions, or whatever
			return f.Apply(args)
		}
	}
	return res
}

func (self *closure) bindArgs(ctx *Scope, args interface{}) {
	vars := self.vars
	for {
		if vars == EMPTY_LIST && args == EMPTY_LIST {
			break
		}
		if vars == EMPTY_LIST {
			ArgumentError(self, args)
		}
		p, pair := vars.(*Pair)
		if args == EMPTY_LIST && pair {
			ArgumentError(self, args)
		}
		if !pair {
			self.bindArg(ctx, vars, args)
			break
		}
		self.bindArg(ctx, p.a, Car(args))
		vars, args = p.d, Cdr(args)
	}
}

func (self *closure) bindArg(ctx *Scope, name, val interface{}) {
	n, ok := name.(Symbol)
	if !ok {
		TypeError("symbol", name)
	}
	ctx.env[n] = val
}

// Macros

func (self *macro) String() string {
	return self.GoString()
}

func (self *macro) GoString() string {
	return "#<macro>"
}
