package lisp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"unsafe"
)

/*
	Basic types
*/

type Symbol string

func (self Symbol) GoString() string {
	return string(self)
}

type Environment map[Symbol]interface{}

type Constant struct {
	str string
}

func NewConstant(str string) interface{} {
	return &Constant{str}
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

func Call(f Function, args ...interface{}) interface{} {
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
			as := lsToVec(args).(Vector)
			if len(as) != l {
				ArgumentError(res, args)
			}
			return f(as)
		})
		return res
	}
	switch f := _f.(type) {
	case func() interface{}:
		return wrap(0, func(args Vector) interface{} {
			return f()
		})
	case func(a interface{}) interface{}:
		return wrap(1, func(args Vector) interface{} {
			return f(args[0])
		})
	case func(a, b interface{}) interface{}:
		return wrap(2, func(args Vector) interface{} {
			return f(args[0], args[1])
		})
	case func(a, b, c interface{}) interface{}:
		return wrap(3, func(args Vector) interface{} {
			return f(args[0], args[1], args[2])
		})
	case func(a, b, c, d interface{}) interface{}:
		return wrap(4, func(args Vector) interface{} {
			return f(args[0], args[1], args[2], args[3])
		})
	case func(a, b, c, d, e interface{}) interface{}:
		return wrap(5, func(args Vector) interface{} {
			return f(args[0], args[1], args[2], args[3], args[4])
		})
	}
	Error(fmt.Sprintf("invalid primitive function: %s", toWrite("%#v", _f)))
	return nil
}

// Takes a map, containing functions to be passed to WrapPrimitive. Returns
// an environment.
func WrapPrimitives(env map[string]interface{}) Environment {
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
	msg  interface{}
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

func WrapError(err interface{}) os.Error {
	switch e := err.(type) {
	case *errorStruct:
		return e
	case os.Error:
		return &errorStruct{Symbol("system-error"), e.String()}
	default:
		TypeError("error", err)
	}
	panic("unreachable")
}

func Throw(kind Symbol, msg interface{}) {
	panic(&errorStruct{kind, msg})
}

func Error(msg string) {
	Throw(Symbol("error"), msg)
}

func TypeError(expected string, obj interface{}) {
	msg := fmt.Sprintf("expecting %s: %s", expected, toWrite("%#v", obj))
	Throw(Symbol("type-error"), msg)
}

func ArgumentError(f, args interface{}) {
	msg := fmt.Sprintf("wrong number of arguments to %v: %#v", f, args)
	Throw(Symbol("argument-error"), msg)
}

func SystemError(err os.Error) {
	Throw(Symbol("system-error"), err.String())
}

func SyntaxError(err string) {
	Throw(Symbol("syntax-error"), err)
}

/*
	Pairs
*/

var EMPTY_LIST = NewConstant("()")

type Pair struct{ a, d interface{} }

func (self *Pair) toWrite(def string) string {
	res := ""
	if self.d == EMPTY_LIST {
		res = fmt.Sprintf("(%s)", toWrite(def, self.a))
	} else if d, ok := self.d.(*Pair); ok {
		res = fmt.Sprintf("(%s %s", toWrite(def, self.a), toWrite(def, d)[1:])
	} else {
		res = fmt.Sprintf("(%s . %s)", toWrite(def, self.a), toWrite(def, self.d))
	}
	return res
}

func (self *Pair) String() string {
	return self.toWrite("%v")
}

func (self *Pair) GoString() string {
	return self.toWrite("%#v")
}

func Cons(a, d interface{}) interface{} {
	return &Pair{a, d}
}

func pairFunc(x interface{}, f func(*Pair) interface{}) interface{} {
	p, ok := x.(*Pair)
	if !ok {
		TypeError("pair", x)
	}
	return f(p)
}

func Car(x interface{}) interface{} {
	return pairFunc(x, func(p *Pair) interface{} { return p.a })
}

func Cdr(x interface{}) interface{} {
	return pairFunc(x, func(p *Pair) interface{} { return p.d })
}

func List(xs ...interface{}) interface{} {
	return vecToLs(Vector(xs))
}

func ListLen(ls interface{}) int {
	res := 0
	for ; ls != EMPTY_LIST; ls, res = Cdr(ls), res+1 {
	}
	return res
}

func ListTail(ls interface{}, idx int) interface{} {
	for ; idx > 0; idx, ls = idx-1, Cdr(ls) {
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

func (self Vector) testRange(i int) {
	if i < 0 || i >= len(self) {
		Error(fmt.Sprintf("invalid index (%v)", i))
	}
}

func (self Vector) Get(i int) interface{} {
	self.testRange(i)
	return self[i]
}

func (self Vector) Set(i int, v interface{}) interface{} {
	self.testRange(i)
	self[i] = v
	return nil
}

func (self Vector) Slice(lo, hi int) interface{} {
	self.testRange(lo)
	self.testRange(hi - 1)
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
	Ports
*/
var (
	EOF_OBJECT   = NewConstant("#eof-object")
	_PORT_CLOSED = os.NewError("port closed")
)

type InputPort struct {
	eof bool
	ref io.Reader
	r   *bufio.Reader
}

func NewInput(r io.Reader) *InputPort {
	if p, ok := r.(*InputPort); ok {
		return p
	}
	return &InputPort{false, r, bufio.NewReader(r)}
}

func (self *InputPort) Read(bs []byte) (int, os.Error) {
	if self.r == nil {
		return 0, _PORT_CLOSED
	}
	if self.eof {
		return 0, os.EOF
	}
	l, err := self.r.Read(bs)
	self.eof = err == os.EOF
	return l, err
}

func (self *InputPort) ReadChar() interface{} {
	if self.r == nil {
		SystemError(_PORT_CLOSED)
	}
	if self.eof {
		return EOF_OBJECT
	}
	res, _, err := self.r.ReadRune()
	if err != nil {
		self.eof = err == os.EOF
		if !self.eof {
			SystemError(err)
		}
	}
	return res
}

func (self *InputPort) ReadByte() interface{} {
	if self.r == nil {
		SystemError(_PORT_CLOSED)
	}
	if self.eof {
		return EOF_OBJECT
	}
	bs := []byte{0}
	_, err := self.Read(bs)
	if err != nil {
		self.eof = err == os.EOF
		if !self.eof {
			SystemError(err)
		}
	}
	return int(bs[0])
}

func (self *InputPort) ReadLine() interface{} {
	if self.r == nil {
		SystemError(_PORT_CLOSED)
	}
	if self.eof {
		return EOF_OBJECT
	}
	res := ""
	for {
		b, _, err := self.r.ReadRune()
		if err != nil {
			self.eof = err == os.EOF
			if !self.eof {
				SystemError(err)
			}
			break
		}
		if b == '\n' {
			break
		}
		res += string(b)
	}
	return res
}

func (self *InputPort) Close() {
	if self.r == nil {
		SystemError(_PORT_CLOSED)
	}
	self.r = nil
	if c, ok := self.ref.(io.Closer); ok {
		err := c.Close()
		if err != nil {
			SystemError(err)
		}
	}
}

func (self *InputPort) Eof() bool {
	return self.eof
}

type OutputPort struct {
	ref io.Writer
	w   *bufio.Writer
}

func NewOutput(w io.Writer) *OutputPort {
	if p, ok := w.(*OutputPort); ok {
		return p
	}
	return &OutputPort{w, bufio.NewWriter(w)}
}

func (self *OutputPort) Write(bs []byte) (int, os.Error) {
	if self.w == nil {
		return 0, _PORT_CLOSED
	}
	return self.w.Write(bs)
}

func (self *OutputPort) WriteString(str string) {
	if self.w == nil {
		SystemError(_PORT_CLOSED)
	}
	_, err := self.w.WriteString(str)
	if err != nil {
		SystemError(err)
	}
}

func (self *OutputPort) WriteByte(b byte) {
	bs := []byte{b}
	_, err := self.Write(bs)
	if err != nil {
		SystemError(err)
	}
}

func (self *OutputPort) Flush() {
	if self.w == nil {
		SystemError(_PORT_CLOSED)
	}
	err := self.w.Flush()
	if err != nil {
		SystemError(err)
	}
}

func (self *OutputPort) Close() {
	if self.w == nil {
		SystemError(_PORT_CLOSED)
	}
	self.w = nil
	if c, ok := self.ref.(io.Closer); ok {
		err := c.Close()
		if err != nil {
			SystemError(err)
		}
	}
}

/*
	Custom types
*/

type Custom struct {
	name Symbol
	val  interface{}
}

func NewCustom(name Symbol, val interface{}) *Custom {
	return &Custom{name, val}
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
