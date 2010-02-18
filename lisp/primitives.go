package lisp

import (
	"io"
	"os"
	"bufio"
	"strings"
	"fmt"
)

// All of the primitive functions defined by the library.
func Primitives() Environment {
	return WrapPrimitives(map[string] interface{} {
		// equality
		"==": eq,
		// syntax
		"read": Read,
		"write": Write,
		"display": Display,
		"macro": newMacro,
		// control
		"load": load,
		"eval": eval,
		"apply": apply,
		"throw": throw,
		"catch": catch,
		"null-environment": nullEnv,
		// type system
		"type-of": typeOf,
		"define-custom-type": newCustom,
		// numbers
		"fixnum->flonum": fixToFlo,
		"fixnum-add": fixnumAdd,
		"fixnum-sub": fixnumSub,
		"fixnum-mul": fixnumMul,
		"fixnum-div": fixnumDiv,
		"fixnum-quotient": quotient,
		"fixnum-modulo": modulo,
		"flonum-add": flonumAdd,
		"flonum-sub": flonumSub,
		"flonum-mul": flonumMul,
		"flonum-div": flonumDiv,
		// strings
		"string-append": stringAppend,
		"string->vector": strToVec,
		// pairs
		"cons": Cons,
		"car": Car,
		"cdr": Cdr,
		"list->vector": lsToVec,
		// vectors
		"make-vector": makeVector,
		"vector-ref": vectorRef,
		"vector-set!": vectorSet,
		"vector-slice": slice,
		"vector->list": vecToLs,
		"vector->string": vecToStr,
		// maps
		//~ "make-map": makeMap,
		//~ "map-ref": mapGet,
		//~ "map-set!": mapSet,
		//~ "in-map?": mapCheck,
		//~ "map-delete": mapDelete,
		// ports
		"read-char": readChar,
		"read-byte": readByte,
		"eof-object?": isEof,
		"write-string": writeString,
		"write-byte": writeByte,
		"flush": flush,
		"close-port": closePort,
	})
}

/*
	Equality
*/

func eq(a, b Any) Any {
	return a == b
}

/*
	Syntax
*/

func newMacro(m Any) Any {
	f, ok := m.(Function)
	if !ok { return TypeError(m) }
	return &macro { f }
}

/* 
	Control
*/

func load(path, env Any) Any {
	ctx, ok := env.(*Context)
	if !ok { return TypeError(env) }
	p, ok := path.(string)
	if !ok { return TypeError(path) }
	return ctx.Load(p)
}

func eval(expr, env Any) Any {
	ctx, ok := env.(*Context)
	if !ok { return TypeError(env) }
	return ctx.Eval(expr)
}

func apply(f, args Any) Any {
	fn, ok := f.(Function)
	if !ok { return TypeError(f) }
	return fn.Apply(args)
}

func throw(kind, msg Any) Any {
	k, ok := kind.(Symbol)
	if !ok { return TypeError(kind) }
	return Throw(k, fmt.Sprintf("%v", msg))
}
		
func catch(thk, hnd Any) Any {
	t, ok := thk.(Function)
	if !ok { return TypeError(t) }
	h, ok := hnd.(Function)
	if !ok { return TypeError(h) }
	res := Call(t)
	// handle any errors
	if r, ok := res.(*errorStruct); ok {
		res = Call(h, r.kind, r.msg)
	}
	return res
}

func nullEnv() Any {
	return NewContext(nil)
}

/*
	Type system
*/

func typeOf(x Any) Any {
	s := "any"
	switch x.(type) {
		case bool: s = "boolean"
		case int: s = "fixnum"
		case float: s = "flonum"
		case string: s = "string"
		case Symbol: s = "symbol"
		case *Pair: s = "pair"
		case Vector: s = "vector"
		case *macro: s = "macro"
		case Function: s = "function"
		case io.ReadWriter: s = "port"
		case io.Reader: s = "input-port"
		case io.Writer: s = "output-port"
		case *Custom: return x.(*Custom).Name()
	}
	if x == nil { s = "void" }
	return Symbol(s)
}

func newCustom(name, fn Any) Any {
	n, ok := name.(Symbol)
	if !ok { return TypeError(name) }
	f, ok := fn.(Function)
	if !ok { return TypeError(fn) }
	wrap := WrapPrimitive(func(x Any) Any { return &Custom { n, x } })
	unwrap := WrapPrimitive(func(x Any) Any {
		c, ok := x.(*Custom)
		if !ok || c.name != n { return TypeError(x) }
		return c.Get()
	})
	set := WrapPrimitive(func(x, v Any) Any {
		c, ok := x.(*Custom)
		if !ok || c.name != n { return TypeError(x) }
		c.Set(v)
		return nil
	})
	res := Call(f, wrap, unwrap, set)
	if Failed(res) { return res }
	return nil
}

/*
	Numbers
*/

func fixToFlo(_x Any) Any {
	switch x := _x.(type) {
		case int: return float(x)
		case float: return x
	}
	return TypeError(_x) 
}

func fixnumAdd(_a, _b Any) Any {
	a, ok := _a.(int)
	if !ok { return TypeError(_a) }
	b, ok := _b.(int)
	if !ok { return TypeError(_b) }
	return a + b
}

func fixnumSub(_a, _b Any) Any {
	a, ok := _a.(int)
	if !ok { return TypeError(_a) }
	b, ok := _b.(int)
	if !ok { return TypeError(_b) }
	return a - b
}

func fixnumMul(_a, _b Any) Any {
	a, ok := _a.(int)
	if !ok { return TypeError(_a) }
	b, ok := _b.(int)
	if !ok { return TypeError(_b) }
	return a * b
}

func fixnumDiv(_a, _b Any) Any {
	a, ok := _a.(int)
	if !ok { return TypeError(_a) }
	b, ok := _b.(int)
	if !ok { return TypeError(_b) }
	if b == 0 { return Error("divide by zero") }
	if a % b == 0 { return a / b }
	return float(a) / float(b)
}

func quotient(_a, _b Any) Any {
	a, ok := _a.(int)
	if !ok { return TypeError(_a) }
	b, ok := _b.(int)
	if !ok { return TypeError(_b) }
	if b == 0 { return Error("divide by zero") }	
	return a / b
}

func modulo(_a, _b Any) Any {
	a, ok := _a.(int)
	if !ok { return TypeError(_a) }
	b, ok := _b.(int)
	if !ok { return TypeError(_b) }
	if b == 0 { return Error("divide by zero") }
	return a % b
}

func flonumAdd(_a, _b Any) Any {
	a, ok := _a.(float)
	if !ok { return TypeError(_a) }
	b, ok := _b.(float)
	if !ok { return TypeError(_b) }
	return a + b
}

func flonumSub(_a, _b Any) Any {
	a, ok := _a.(float)
	if !ok { return TypeError(_a) }
	b, ok := _b.(float)
	if !ok { return TypeError(_b) }
	return a - b
}

func flonumMul(_a, _b Any) Any {
	a, ok := _a.(float)
	if !ok { return TypeError(_a) }
	b, ok := _b.(float)
	if !ok { return TypeError(_b) }
	return a * b
}

func flonumDiv(_a, _b Any) Any {
	a, ok := _a.(float)
	if !ok { return TypeError(_a) }
	b, ok := _b.(float)
	if !ok { return TypeError(_b) }
	if b == 0 { return Error("divide by zero") }
	return a / b
}

/*
	Strings
*/

func stringAppend(_a, _b Any) Any {
	a, ok := _a.(string)
	if !ok { return TypeError(_a) }
	b, ok := _b.(string)
	if !ok { return TypeError(_b) }
	return a + b
}

func strToVec(str Any) Any {
	s, ok := str.(string)
	if !ok { return TypeError(str) }
	rs := strings.Runes(s)
	res := make(Vector, len(rs))
	for i, x := range rs { res[i] = x }
	return res
}

/*
	Vectors
*/

func makeVector(size, fill Any) Any {
	s, ok := size.(int)
	if !ok { return TypeError(size) }
	res := make(Vector, s)
	for i,_ := range res {
		res[i] = fill
	}
	return res
}

func vectorRef(vec, idx Any) Any {
	v, ok := vec.(Vector)
	if !ok { return TypeError(vec) }
	i, ok := idx.(int)
	if !ok { return TypeError(idx) }
	return v.Get(i)
}

func vectorSet(vec, idx, val Any) Any {
	v, ok := vec.(Vector)
	if !ok { return TypeError(vec) }
	i, ok := idx.(int)
	if !ok { return TypeError(idx) }
	return v.Set(i, val)
}

func slice(vec, lo, hi Any) Any {
	v, ok := vec.(Vector)
	if !ok { return TypeError(vec) }
	l, ok := lo.(int)
	if !ok { return TypeError(lo) }
	h, ok := hi.(int)
	if !ok { return TypeError(hi) }
	return v[l:h]
}

func lsToVec(lst Any) Any {
	l := ListLen(lst)
	if l == -1 { return TypeError(lst) }
	res := make(Vector, l)
	for i := 0; lst != EMPTY_LIST; i, lst = i + 1, Cdr(lst) {
		a := Car(lst)
		if Failed(a) { return a }
		res[i] = a
	}
	return res
}

func vecToLs(vec Any) Any {
	xs, ok := vec.(Vector)
	if !ok { return TypeError(vec) }
	var res Any = EMPTY_LIST
	for i := len(xs) - 1; i >= 0; i-- {
		res = Cons(xs[i], res)
	}
	return res
}

func vecToStr(vec Any) Any {
	cs, ok := vec.(Vector)
	if !ok { return TypeError(vec) }
	res := make([]int, len(cs))
	for i, c := range cs {
		r, ok := c.(int)
		if !ok { return TypeError(c) }
		res[i] = r
	}
	return string(res)
}

/*
	Ports
*/

var EOF_OBJECT = NewConstant("#eof-object")

func readChar(port Any) Any {
	p, ok := port.(*bufio.Reader)
	if !ok { return TypeError(port) }
	r, _, err := p.ReadRune()
	if err != nil {
		if err == os.EOF { return EOF_OBJECT }
		return SystemError(err)
	}
	return string(r)
}

func readByte(port Any) Any {	
	p, ok := port.(io.Reader)
	if !ok { return TypeError(port) }
	buf := make([]byte, 1)
	_, err := p.Read(buf); 		
	if err != nil { 
		if err == os.EOF { return EOF_OBJECT }
		return SystemError(err)
	}
	return int(buf[0])
}

func isEof(x Any) Any {
	return x == EOF_OBJECT
}

func writeString(port, str Any) Any {
	p, ok := port.(*bufio.Writer)
	if !ok { return TypeError(port) }
	s, ok := str.(string)
	if !ok { return TypeError(str) }
	_, err := p.WriteString(s)
	if err != nil { return SystemError(err) }
	return nil
}

func writeByte(port, bte Any) Any {	
	p, ok := port.(io.Writer)
	if !ok { return TypeError(port) }
	b, ok := bte.(int)
	if !ok { return TypeError(bte) }
	_, err := p.Write([]byte { byte(b) })
	if err != nil { return SystemError(err) }
	return nil
}

func flush(port Any) Any {
	p, ok := port.(*bufio.Writer)
	if !ok { return TypeError(port) }
	err := p.Flush()
	if err != nil { return SystemError(err) }
	return nil
}

func closePort(port Any) Any {
	p, ok := port.(io.Closer)
	if !ok { return TypeError(port) }
	err := p.Close()
	if err != nil { return SystemError(err) }
	return nil
}

