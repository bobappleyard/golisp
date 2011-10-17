package lisp

import (
	"big"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	
	"github.com/bobappleyard/bwl/errors"
)

// All of the primitive functions defined by the library.
func Primitives() Environment {
	return WrapPrimitives(map[string] interface{} {
		// equality
		"==": eq,
		// syntax
		"read": Read,
		"read-file": ReadFile,
		"read-string": readStr,
		"write": Write,
		"display": Display,
		"macro": newMacro,
		// control
		"go": spawn,
		"load": load,
		"eval": eval,
		"apply": apply,
		"throw": throw,
		"catch": catch,
		"null-environment": nullEnv,
		"capture-environment": capEnv,
		"start-process": startProc,
		// type system
		"type-of": typeOf,
		"define-type": newCustom,
		// symbols
		"symbol->string": symToStr,
		"string->symbol": strToSym,
		"gensym": gensym,
		// numbers
		"fixnum->flonum": fixToFlo,
		"fixnum-add": fixnumAdd,
		"fixnum-sub": fixnumSub,
		"fixnum-mul": fixnumMul,
		"fixnum-div": fixnumDiv,
		"quotient": fixnumQuotient,
		"remainder": fixnumRemainder,
		"modulo": fixnumModulo,
		"flonum-add": flonumAdd,
		"flonum-sub": flonumSub,
		"flonum-mul": flonumMul,
		"flonum-div": flonumDiv,
		// strings
		"string-split": stringSplit,
		"string-join": stringJoin,
		"string->vector": strToVec,
		"object->string": objToStr,
		// pairs
		"cons": Cons,
		"car": Car,
		"cdr": Cdr,
		"list->vector": lsToVec,
		// vectors
		"make-vector": makeVector,
		"vector-length": vectorLength,
		"vector-ref": vectorRef,
		"vector-set!": vectorSet,
		"vector-slice": slice,
		"vector->list": vecToLs,
		"vector->string": vecToStr,
		// ports
		"open-file": openFile,
		"read-char": readChar,
		"read-byte": readByte,
		"eof-object?": isEof,
		"write-string": writeString,
		"write-byte": writeByte,
		"flush": flush,
		"close": closePort,
		// channels
		"make-channel": makeChannel,
		"channel-send": send,
		"channel-receive": receive,
	})
}

/*
	Equality
*/

func eq(a, b interface{}) interface{} {
	var res interface{}
	errors.Catch(
		func() { res = a == b }, 
		func(_ interface{}) { res = false },
	)
	return res
}

/*
	Syntax
*/

func readStr(str interface{}) interface{} {
	s, ok := str.(string)
	if !ok { TypeError("string", str) }
	return ReadString(s)
}

func newMacro(m interface{}) interface{} {
	f, ok := m.(Function)
	if !ok { TypeError("function", m) }
	return &macro { f }
}

/* 
	Control
*/

func spawn(f interface{}) interface{} {
	fn, ok := f.(Function)
	if !ok { TypeError("function", f) }
	go fn.Apply(EMPTY_LIST)
	return nil
}

func load(path, env interface{}) interface{} {
	ctx, ok := env.(*Scope)
	if !ok { TypeError("environment", env) }
	p, ok := path.(string)
	if !ok { TypeError("string", path) }
	ctx.Load(p)
	return nil
}

func eval(expr, env interface{}) interface{} {
	ctx, ok := env.(*Scope)
	if !ok { TypeError("environment", env) }
	return ctx.Eval(expr)
}

func apply(f, args interface{}) interface{} {
	fn, ok := f.(Function)
	if !ok { TypeError("function", f) }
	return fn.Apply(args)
}

func throw(kind, msg interface{}) interface{} {
	k, ok := kind.(Symbol)
	if !ok { TypeError("symbol", kind) }
	Throw(k, msg)
	panic("unreachable")
}
		
func catch(thk, hnd interface{}) interface{} {
	t, ok := thk.(Function)
	if !ok { TypeError("function", t) }
	h, ok := hnd.(Function)
	if !ok { TypeError("function", h) }
	var res interface{}
	errors.Catch(
		func() { res = Call(t) },
		func(err interface{}) {	
			e := WrapError(err).(*errorStruct)
			res = Call(h, e.kind, e.msg) 
		},
	)
	return res
}

func nullEnv() interface{} {
	return NewScope(nil)
}

func capEnv(env interface{}) interface{} {
	e, ok := env.(*Scope)
	if !ok { TypeError("environment", env) }
	return NewScope(e)
}

func startProc(path, args interface{}) interface{} {
	p, ok := path.(string)
	if !ok { TypeError("string", path) }
	argv := make([]string, ListLen(args))
	for cur, i := args, 0; cur != EMPTY_LIST; cur, i = Cdr(cur), i + 1 {
		x := Car(cur)
		s, ok := x.(string)
		if !ok { TypeError("string", x) }
		argv[i] = s
	}
	inr, inw, err := os.Pipe()
	if err != nil { SystemError(err) }
	outr, outw, err := os.Pipe()
	if err != nil { SystemError(err) }
	_, err = os.ForkExec(p, argv, os.Envs, "", []*os.File { inr, outw, os.Stderr })
	if err != nil { SystemError(err) }
	return Cons(NewOutput(inw), NewInput(outr))
}

/*
	Type system
*/

func typeOf(x interface{}) interface{} {
	s := "unknown"
	switch x.(type) {
		case bool: s = "boolean"
		case int: s = "fixnum"
		case float32: s = "flonum"
		case string: s = "string"
		case Symbol: s = "symbol"
		case *Pair: s = "pair"
		case Vector: s = "vector"
		case *macro: s = "macro"
		case Function: s = "function"
		case *InputPort: s = "input-port"
		case *OutputPort: s = "output-port"
		case *Custom: return x.(*Custom).Name()
		case *big.Int: s = "bignum"
		case chan interface{}: s = "channel"
	}
	if x == nil { s = "void" }
	return Symbol(s)
}

func newCustom(name, fn interface{}) interface{} {
	n, ok := name.(Symbol)
	if !ok { TypeError("symbol", name) }
	f, ok := fn.(Function)
	if !ok { TypeError("function", fn) }
	wrap := WrapPrimitive(func(x interface{}) interface{} { 
		return NewCustom(n, x) 
	})
	unwrap := WrapPrimitive(func(x interface{}) interface{} {
		c, ok := x.(*Custom)
		if !ok || c.name != n { TypeError(string(n), x) }
		return c.Get()
	})
	set := WrapPrimitive(func(x, v interface{}) interface{} {
		c, ok := x.(*Custom)
		if !ok || c.name != n { TypeError(string(n), x) }
		c.Set(v)
		return nil
	})
	Call(f, wrap, unwrap, set)
	return nil
}

/*
	Symbols
*/

func symToStr(sym interface{}) interface{} {
	s, ok := sym.(Symbol)
	if !ok { TypeError("symbol", sym) }
	return string(s)
}

func strToSym(str interface{}) interface{} {
	s, ok := str.(string)
	if !ok { TypeError("string", str) }
	return Symbol(s)
}

var gensyms = func() <-chan Symbol {
	syms := make(chan Symbol)
	go func() {
		i := 0
		for {
			syms <- Symbol("#gensym" + strconv.Itoa(i))
			i++
		}
	}()
	return syms
}()

func gensym() interface{} {
	return <-gensyms
}

/*
	Numbers
*/

func fixToFlo(_x interface{}) interface{} {
	switch x := _x.(type) {
		case int: return float32(x)
		case float32: return x
	}
	TypeError("number", _x) 
	panic("unreachable")
}

func fixnumFunc(_a, _b interface{}, f func (a, b int) interface{}) interface{} {
	a, ok := _a.(int)
	if !ok { TypeError("fixnum", _a) }
	b, ok := _b.(int)
	if !ok { TypeError("fixnum", _b) }
	return f(a, b)
}

func fixnumAdd(_a, _b interface{}) interface{} {
	return fixnumFunc(_a, _b, func(a, b int) interface{} { return a + b })
}

func fixnumSub(_a, _b interface{}) interface{} {
	return fixnumFunc(_a, _b, func(a, b int) interface{} { return a - b })
}

func fixnumMul(_a, _b interface{}) interface{} {
	return fixnumFunc(_a, _b, func(a, b int) interface{} { return a * b })
}

func fixnumDiv(_a, _b interface{}) interface{} {
	return fixnumFunc(_a, _b, func(a, b int) interface{} {
		if b == 0 { Error("divide by zero") }
		if a % b == 0 { return a / b }
		return float32(a) / float32(b)
	})
}

func fixnumQuotient(_a, _b interface{}) interface{} {
	return fixnumFunc(_a, _b, func(a, b int) interface{} {
		if b == 0 { Error("divide by zero") }	
		return a / b
	})
}

func fixnumRemainder(_a, _b interface{}) interface{} {
	return fixnumFunc(_a, _b, func(a, b int) interface{} {
		if b == 0 { Error("divide by zero") }
		return a % b
	})
}

func fixnumModulo(_a, _b interface{}) interface{} {
	return fixnumFunc(_a, _b, func(a, b int) interface{} {
		if b == 0 { Error("divide by zero") }
		r := a % b
		if !(r == 0 || (a > 0 && b > 0) || (a < 0 && b < 0)) {
			r += b
		}
		return r
	})
}

func flonumFunc(_a, _b interface{}, f func(a, b float32) interface{}) interface{} {
	a, ok := _a.(float32)
	if !ok { TypeError("flonum", _a) }
	b, ok := _b.(float32)
	if !ok { TypeError("flonum", _b) }
	return f(a, b)
}

func flonumAdd(_a, _b interface{}) interface{} {
	return flonumFunc(_a, _b, func(a, b float32) interface{} { return a + b })
}

func flonumSub(_a, _b interface{}) interface{} {
	return flonumFunc(_a, _b, func(a, b float32) interface{} { return a - b })
}

func flonumMul(_a, _b interface{}) interface{} {
	return flonumFunc(_a, _b, func(a, b float32) interface{} { return a * b })
}

func flonumDiv(_a, _b interface{}) interface{} {
	return flonumFunc(_a, _b, func(a, b float32) interface{} {
		if b == 0 { Error("divide by zero") }
		return a / b
	})
}

/*
	Strings
*/

func stringSplit(str, sep interface{}) interface{} {
	s, ok := str.(string)
	if !ok { TypeError("string", str) }
	b, ok := sep.(string)
	if !ok { TypeError("string", sep) }
	ss := strings.Split(s, b, 0)
	res := EMPTY_LIST
	for i := len(ss) - 1; i >= 0; i-- {
		res = Cons(ss[i], res)
	}
	return res
}

func stringJoin(strs, sep interface{}) interface{} {
	ss := make([]string, ListLen(strs))
	b, ok := sep.(string)
	if !ok { TypeError("string", sep) }
	for cur, i := strs, 0; cur != EMPTY_LIST; cur, i = Cdr(cur), i + 1 {
		x := Car(cur)
		s, ok := x.(string)
		if !ok { TypeError("string", x) }
		ss[i] = s
	}
	return strings.Join(ss, b)
}

func strToVec(str interface{}) interface{} {
	s, ok := str.(string)
	if !ok { TypeError("string", str) }
	rs := bytes.Runes([]byte(s))
	res := make(Vector, len(rs))
	for i, x := range rs { res[i] = x }
	return res
}

func objToStr(obj interface{}) interface{} {
	return toWrite("%v", obj)
}

/*
	Vectors
*/

func makeVector(size, fill interface{}) interface{} {
	s, ok := size.(int)
	if !ok { TypeError("vector", size) }
	res := make(Vector, s)
	for i,_ := range res {
		res[i] = fill
	}
	return res
}

func vectorLength(vec interface{}) interface{} {
	v, ok := vec.(Vector)
	if !ok { TypeError("vector", vec) }
	return len(v)
}

func vectorRef(vec, idx interface{}) interface{} {
	v, ok := vec.(Vector)
	if !ok { TypeError("vector", vec) }
	i, ok := idx.(int)
	if !ok { TypeError("fixnum", idx) }
	return v.Get(i)
}

func vectorSet(vec, idx, val interface{}) interface{} {
	v, ok := vec.(Vector)
	if !ok { TypeError("vector", vec) }
	i, ok := idx.(int)
	if !ok { TypeError("fixnum", idx) }
	return v.Set(i, val)
}

func slice(vec, lo, hi interface{}) interface{} {
	v, ok := vec.(Vector)
	if !ok { TypeError("vector", vec) }
	l, ok := lo.(int)
	if !ok { TypeError("fixnum", lo) }
	h, ok := hi.(int)
	if !ok { TypeError("fixnum", hi) }
	if l < 0 { Error(fmt.Sprintf("invalid index (%v)", h)) }
	if h > len(v) { Error(fmt.Sprintf("invalid index (%v)", h)) }
	return v[l:h]
}

func lsToVec(lst interface{}) interface{} {
	l := ListLen(lst)
	if l == -1 { TypeError("pair", lst) }
	res := make(Vector, l)
	for i := 0; lst != EMPTY_LIST; i, lst = i + 1, Cdr(lst) {
		a := Car(lst)
		if Failed(a) { return a }
		res[i] = a
	}
	return res
}

func vecToLs(vec interface{}) interface{} {
	xs, ok := vec.(Vector)
	if !ok { TypeError("vector", vec) }
	var res interface{} = EMPTY_LIST
	for i := len(xs) - 1; i >= 0; i-- {
		res = Cons(xs[i], res)
	}
	return res
}

func vecToStr(vec interface{}) interface{} {
	cs, ok := vec.(Vector)
	if !ok { TypeError("vector", vec) }
	res := make([]int, len(cs))
	for i, c := range cs {
		r, ok := c.(int)
		if !ok { TypeError("vector of fixnums", vec) }
		res[i] = r
	}
	return string(res)
}

/*
	Ports
*/

func openFile(path, mode interface{}) interface{} {
	p, ok := path.(string)
	if !ok { TypeError("string", path) }
	m, ok := mode.(Symbol)
	if !ok { TypeError("symbol", mode) }
	wrap := func(x interface{}) interface{} { return NewOutput(x.(io.Writer)) }
	filemode, perms := 0, 0
	switch string(m) {
		case "create": filemode, perms = os.O_CREAT, 0644
		case "read": filemode, wrap = os.O_RDONLY, func(x interface{}) interface{} {
			return NewInput(x.(io.Reader))
		}
		case "write": filemode = os.O_WRONLY
		case "append": filemode = os.O_APPEND
		default: Error(fmt.Sprintf("wrong access token: %s", m))
	}
	f, err := os.Open(p, filemode, perms)
	if err != nil { SystemError(err) }
	return wrap(f)
}

func readChar(port interface{}) interface{} {
	p, ok := port.(*InputPort)
	if !ok { TypeError("input-port", port) }
	return p.ReadChar()
}

func readByte(port interface{}) interface{} {	
	p, ok := port.(*InputPort)
	if !ok { TypeError("input-port", port) }
	return p.ReadByte()
}

func isEof(x interface{}) interface{} {
	return x == EOF_OBJECT
}

func writeString(port, str interface{}) interface{} {
	p, ok := port.(*OutputPort)
	if !ok { TypeError("output-port", port) }
	s, ok := str.(string)
	if !ok { TypeError("string", str) }
	p.WriteString(s)
	return nil
}

func writeByte(port, bte interface{}) interface{} {	
	p, ok := port.(*OutputPort)
	if !ok { TypeError("output-port", port) }
	b, ok := bte.(int)
	if !ok { TypeError("fixnum", bte) }
	p.WriteByte(byte(b))
	return nil
}

func flush(port interface{}) interface{} {
	p, ok := port.(*OutputPort)
	if !ok { TypeError("output-port", port) }
	p.Flush()
	return nil
}

func closePort(port interface{}) interface{} {
	switch p := port.(type) {
		case *InputPort: p.Close()
		case *OutputPort: p.Close()
		default: TypeError("port", port)
	}
	return nil
}

/*
	Channels
*/

func makeChannel() interface{} {
	return make(chan interface{})
}

func send(ch, v interface{}) interface{} {
	channel, ok := ch.(chan interface{})
	if !ok { TypeError("channel", ch) }
	channel <- v
	return nil
}

func receive(ch interface{}) interface{} {
	channel, ok := ch.(chan interface{})
	if !ok { TypeError("channel", ch) }
	return <- channel
}

