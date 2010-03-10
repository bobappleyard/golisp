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
// lisp system. Crashes if it fails to match, which I suppose is pretty
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
	errors.Fatal(os.ErrorString("invalid primitive function"))
	return nil
}

// Takes a map, containing functions to be passed to WrapPrimitive. Returns 
// an environment. Will crash the program if any fail to match. Consider 
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
	msg Any
}

func (self *errorStruct) String() string {
	return self.GoString()
}

func (self *errorStruct) GoString() string {
	return fmt.Sprintf("%v: %s", self.kind, toWrite("%v", self.msg))
}

func Failed(x Any) bool {
	_, failed := x.(*errorStruct)
	return failed
}

func Throw(kind Symbol, msg Any) os.Error {
	return &errorStruct { kind, msg }
}

func Error(msg string) os.Error {
	return Throw(Symbol("error"), msg)
}

func TypeError(expected string, obj Any) os.Error {
	return Throw(
		Symbol("type-error"), 
		fmt.Sprintf("expecting %s: %s", expected, toWrite("%#v", obj)),
	)
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

/*
	Pairs
*/

var EMPTY_LIST = NewConstant("()")

type Pair struct { a, d Any }

func (self *Pair) toWrite(def string) string {
	if self.d == EMPTY_LIST {
		return fmt.Sprintf("(%s)", toWrite(def, self.a))
	}
	if d, ok := self.d.(*Pair); ok {
		return fmt.Sprintf("(%s %s", toWrite(def, self.a), d.String()[1:])
	}
	return fmt.Sprintf("(%s . %s)", toWrite(def, self.a), toWrite(def, self.d))
}

func (self *Pair) String() string {
	return self.toWrite("%v")
}

func (self *Pair) GoString() string {
	return self.toWrite("%#v")
}

func Cons(a, d Any) Any {
	if Failed(a) { return a }
	if Failed(d) { return d }
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

func (self Vector) testRange(i int) Any {
	if i < 0 || i >= len(self) { 
		return Error(fmt.Sprintf("invalid index (%v)", i))
	}
	return nil
}

func (self Vector) Get(i int) Any {
	if e := self.testRange(i); e != nil { return e }
	return self[i]
}

func (self Vector) Set(i int, v Any) Any {
	if e := self.testRange(i); e != nil { return e }
	self[i] = v
	return nil
}

func (self Vector) Slice(lo, hi int) Any {
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
	val Any
}

func NewCustom(name Symbol, val Any) *Custom {
	return &Custom { name, val }
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

func (self *Custom) String() string {
	return self.GoString()
}

func (self *Custom) GoString() string {
	_, addr := unsafe.Reflect(self)
	return fmt.Sprintf("#<%s: %x>", self.name, uintptr(addr))
}
