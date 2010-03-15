package lisp

import (
	"fmt"
	"os"
	"unsafe"
	"./errors"
)

/*
	Basic types
*/

type Symbol string

func (self Symbol) GoString() string {
	return string(self)
}

type Environment map[Symbol] interface{}

type Constant struct { 
	str string
}

func NewConstant(str string) interface{} {
	return &Constant { str }
}

func (self *Constant) String() string {
	return self.GoString()
}

func (self *Constant) GoString() string {
	return self.str
}

// interface{}thing except the boolean value false counts as true.
func True(x interface{}) bool {
	if b, ok := x.(bool); ok {
		return b
	}
	return true
}

/*
	Functions
*/

type Function interface {
	Apply(args interface{}) interface{}
}

func Call(f Function, args... interface{}) interface{} {
	return f.Apply(vecToLs(Vector(args)))
}

type Primitive func(args interface{}) interface{}

func (self Primitive) Apply(args interface{}) interface{} {
	return self(args)
}

func (self Primitive) String() string {
	return self.GoString()
}

func (self Primitive) GoString() string {
	return "#<primitive>"
}

// Takes a function, which can take interface{}thing from none to five lisp.interface{} and 
// must return lisp.interface{}, and returns a function that can be called by the
// lisp system. Crashes if it fails to match, which I suppose is pretty
// bad, really.
func WrapPrimitive(_f interface{}) Function {
	wrap := func(l int, f func(Vector) interface{}) Function {
		var res Function
		res = Primitive(func(args interface{}) interface{} {
			as, ok := lsToVec(args).(Vector)
			if !ok { return ArgumentError(res, args) }
			if len(as) != l { return ArgumentError(res, args) }
			return f(as)
		})
		return res
	}
	switch f := _f.(type) {
		case func() interface{}: return wrap(0, func(args Vector) interface{} { 
			return f() 
		})
		case func(a interface{}) interface{}: return wrap(1, func(args Vector) interface{} {
			return f(args[0])
		})
		case func(a, b interface{}) interface{}: return wrap(2, func(args Vector) interface{} { 
			return f(args[0], args[1])
		})
		case func(a, b, c interface{}) interface{}: return wrap(3, func(args Vector) interface{} { 
			return f(args[0], args[1], args[2])
		})
		case func(a, b, c, d interface{}) interface{}: return wrap(4, func(args Vector) interface{} { 
			return f(args[0], args[1], args[2], args[3])
		})
		case func(a, b, c, d, e  interface{}) interface{}: return wrap(5, func(args Vector) interface{} { 
			return f(args[0], args[1], args[2], args[3], args[4])
		})
	}
	errors.Fatal(os.ErrorString("invalid primitive function"))
	return nil
}

// Takes a map, containing functions to be passed to WrapPrimitive. Returns 
// an environment. Will crash the program if interface{} fail to match. Consider 
// yourself warned.
func WrapPrimitives(env map[string] interface{}) Environment {
	res := make(Environment)
	for k, v := range env {
		res[Symbol(k)] = WrapPrimitive(v)
	}
	return res
}

/*
	Errors
*/

type errorStruct struct {
	kind Symbol
	msg interface{}
}

func (self *errorStruct) String() string {
	return self.GoString()
}

func (self *errorStruct) GoString() string {
	return fmt.Sprintf("%v: %s", self.kind, toWrite("%v", self.msg))
}

func Failed(x interface{}) bool {
	_, failed := x.(*errorStruct)
	return failed
}

func Throw(kind Symbol, msg interface{}) os.Error {
	return &errorStruct { kind, msg }
}

func Error(msg string) os.Error {
	return Throw(Symbol("error"), msg)
}

func TypeError(expected string, obj interface{}) os.Error {
	return Throw(
		Symbol("type-error"), 
		fmt.Sprintf("expecting %s: %s", expected, toWrite("%#v", obj)),
	)
}

func ArgumentError(f, args interface{}) os.Error {
	msg := fmt.Sprintf("wrong number of arguments to %v: %#v", f, args)
	return Throw(Symbol("argument-error"), msg)
}

func SystemError(err os.Error) os.Error {
	return Throw(Symbol("system-error"), err.String())
}

func SyntaxError(err string) os.Error {
	return Throw(Symbol("syntax-error"), err)
}

/*
	Pairs
*/

var EMPTY_LIST = NewConstant("()")

type Pair struct { a, d interface{} }

func (self *Pair) toWrite(def string) string {
	if self.d == EMPTY_LIST {
		return fmt.Sprintf("(%s)", toWrite(def, self.a))
	}
	if d, ok := self.d.(*Pair); ok {
		return fmt.Sprintf("(%s %s", toWrite(def, self.a), toWrite(def, d)[1:])
	}
	return fmt.Sprintf("(%s . %s)", toWrite(def, self.a), toWrite(def, self.d))
}

func (self *Pair) String() string {
	return self.toWrite("%v")
}

func (self *Pair) GoString() string {
	return self.toWrite("%#v")
}

func Cons(a, d interface{}) interface{} {
	if Failed(a) { return a }
	if Failed(d) { return d }
	return &Pair { a, d }
}

func pairFunc(x interface{}, f func(*Pair) interface{}) interface{} {
	if Failed(x) { return x }
	p, ok := x.(*Pair)
	if !ok { return TypeError("pair", x) }
	return f(p)
}

func Car(x interface{}) interface{} {
	return pairFunc(x, func(p *Pair) interface{} { return p.a })
}

func Cdr(x interface{}) interface{} {
	return pairFunc(x, func(p *Pair) interface{} { return p.d })
}

func List(xs... interface{}) interface{} {
	return vecToLs(Vector(xs))
}

func ListLen(ls interface{}) int {
	res := 0
	for; ls != EMPTY_LIST; ls, res = Cdr(ls), res + 1 {
		if Failed(ls) { return -1 }
	}
	return res
}

func ListTail(ls interface{}, idx int) interface{} {
	for ; idx > 0; idx, ls = idx - 1, Cdr(ls) {
		if Failed(ls) { return ls }
	}
	return ls
}

func ListRef(ls interface{}, idx int) interface{} {
	return Car(ListTail(ls, idx))
}

/*
	Vectors
*/

type Vector []interface{}

func (self Vector) testRange(i int) interface{} {
	if i < 0 || i >= len(self) { 
		return Error(fmt.Sprintf("invalid index (%v)", i))
	}
	return nil
}

func (self Vector) Get(i int) interface{} {
	if e := self.testRange(i); e != nil { return e }
	return self[i]
}

func (self Vector) Set(i int, v interface{}) interface{} {
	if e := self.testRange(i); e != nil { return e }
	self[i] = v
	return nil
}

func (self Vector) Slice(lo, hi int) interface{} {
	if e := self.testRange(lo); e != nil { return e }
	if e := self.testRange(hi-1); e != nil { return e }
	return self[lo:hi]
}

func (self Vector) toWrite(def string) string {
	res := "#("
	for _, x := range self {
		res += toWrite(def, x) + " "
	}
	return res[0:len(res)-1] + ")"
}

func (self Vector) String() string {
	return self.toWrite("%v")
}

func (self Vector) GoString() string {
	return self.toWrite("%#v")
}

/*
	Custom types
*/

type Custom struct {
	name Symbol
	val interface{}
}

func NewCustom(name Symbol, val interface{}) *Custom {
	return &Custom { name, val }
}

func (self *Custom) Name() Symbol {
	return self.name
}

func (self *Custom) Get() interface{} {
	return self.val
}

func (self *Custom) Set(val interface{}) {
	self.val = val
}

func (self *Custom) String() string {
	return self.GoString()
}

func (self *Custom) GoString() string {
	_, addr := unsafe.Reflect(self)
	return fmt.Sprintf("#<%s: %x>", self.name, uintptr(addr))
}
