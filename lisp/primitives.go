package lisp

import (
	"io"
	"os"
	"bufio"
	"bytes"
	"fmt"
	"strconv"
)

// All of the primitive functions defined by the library.
func Primitives() Environment {
	return WrapPrimitives(map[string] interface{} {
		// equality
		"==": eq,
		// syntax
		"read": Read,
		"read-string": readStr,
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
		"fixnum-quotient": quotient,
		"fixnum-modulo": modulo,
		"flonum-add": flonumAdd,
		"flonum-sub": flonumSub,
		"flonum-mul": flonumMul,
		"flonum-div": flonumDiv,
		// strings
		"string-append": stringAppend,
		"string->vector": strToVec,
		"object->string": objToStr,
		// pairs
		"cons": Cons,
		"car": Car,
		"cdr": Cdr,
		"set-car!": setCar,
		"set-cdr!": setCdr,
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

func readStr(str Any) Any {
	s, ok := str.(string)
	if !ok { return TypeError("string", str) }
	return ReadString(s)
}

func newMacro(m Any) Any {
	f, ok := m.(Function)
	if !ok { return TypeError("function", m) }
	return &macro { f }
}

/* 
	Control
*/

func load(path, env Any) Any {
	ctx, ok := env.(*Scope)
	if !ok { return TypeError("environment", env) }
	p, ok := path.(string)
	if !ok { return TypeError("string", path) }
	return ctx.Load(p)
}

func eval(expr, env Any) Any {
	ctx, ok := env.(*Scope)
	if !ok { return TypeError("environment", env) }
	return ctx.Eval(expr)
}

func apply(f, args Any) Any {
	fn, ok := f.(Function)
	if !ok { return TypeError("function", f) }
	return fn.Apply(args)
}

func throw(kind, msg Any) Any {
	k, ok := kind.(Symbol)
	if !ok { return TypeError("symbol", kind) }
	return Throw(k, fmt.Sprintf("%v", msg))
}
		
func catch(thk, hnd Any) Any {
	t, ok := thk.(Function)
	if !ok { return TypeError("function", t) }
	h, ok := hnd.(Function)
	if !ok { return TypeError("function", h) }
	res := Call(t)
	// handle any errors
	if r, ok := res.(*errorStruct); ok {
		res = Call(h, r.kind, r.msg)
	}
	return res
}

func nullEnv() Any {
	return NewScope(nil)
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
	if !ok { return TypeError("symbol", name) }
	f, ok := fn.(Function)
	if !ok { return TypeError("function", fn) }
	wrap := WrapPrimitive(func(x Any) Any { return &Custom { n, x } })
	unwrap := WrapPrimitive(func(x Any) Any {
		c, ok := x.(*Custom)
		if !ok || c.name != n { return TypeError(string(n), x) }
		return c.Get()
	})
	set := WrapPrimitive(func(x, v Any) Any {
		c, ok := x.(*Custom)
		if !ok || c.name != n { return TypeError(string(n), x) }
		c.Set(v)
		return nil
	})
	res := Call(f, wrap, unwrap, set)
	if Failed(res) { return res }
	return nil
}

/*
	Symbols
*/

func symToStr(sym Any) Any {
	s, ok := sym.(Symbol)
	if !ok { return TypeError("symbol", sym) }
	return string(s)
}

func strToSym(str Any) Any {
	s, ok := str.(string)
	if !ok { return TypeError("string", str) }
	return Symbol(s)
}

var gensyms = func() <-chan Symbol {
	res := make(chan Symbol)
	go func() {
		i := 0
		for {
			res <- Symbol("#gensym" + strconv.Itoa(i))
			i++
		}
	}()
	return res
}()

func gensym() Any {
	return <-gensyms
}

/*
	Numbers
*/

func fixToFlo(_x Any) Any {
	switch x := _x.(type) {
		case int: return float(x)
		case float: return x
	}
	return TypeError("number", _x) 
}

func fixnumFunc(_a, _b Any, f func (a, b int) Any) Any {
	a, ok := _a.(int)
	if !ok { return TypeError("fixnum", _a) }
	b, ok := _b.(int)
	if !ok { return TypeError("fixnum", _b) }
	return f(a, b)
}

func fixnumAdd(_a, _b Any) Any {
	return fixnumFunc(_a, _b, func(a, b int) Any { return a + b })
}

func fixnumSub(_a, _b Any) Any {
	return fixnumFunc(_a, _b, func(a, b int) Any { return a - b })
}

func fixnumMul(_a, _b Any) Any {
	return fixnumFunc(_a, _b, func(a, b int) Any { return a * b })
}

func fixnumDiv(_a, _b Any) Any {
	return fixnumFunc(_a, _b, func(a, b int) Any {
		if b == 0 { return Error("divide by zero") }
		if a % b == 0 { return a / b }
		return float(a) / float(b)
	})
}

func quotient(_a, _b Any) Any {
	return fixnumFunc(_a, _b, func(a, b int) Any {
		if b == 0 { return Error("divide by zero") }	
		return a / b
	})
}

func modulo(_a, _b Any) Any {
	return fixnumFunc(_a, _b, func(a, b int) Any {
		if b == 0 { return Error("divide by zero") }
		return a % b
	})
}

func flonumFunc(_a, _b Any, f func(a, b float) Any) Any {
	a, ok := _a.(float)
	if !ok { return TypeError("flonum", _a) }
	b, ok := _b.(float)
	if !ok { return TypeError("flonum", _b) }
	return f(a, b)
}

func flonumAdd(_a, _b Any) Any {
	return flonumFunc(_a, _b, func(a, b float) Any { return a + b })
}

func flonumSub(_a, _b Any) Any {
	return flonumFunc(_a, _b, func(a, b float) Any { return a - b })
}

func flonumMul(_a, _b Any) Any {
	return flonumFunc(_a, _b, func(a, b float) Any { return a * b })
}

func flonumDiv(_a, _b Any) Any {
	return flonumFunc(_a, _b, func(a, b float) Any {
		if b == 0 { return Error("divide by zero") }
		return a / b
	})
}

/*
	Strings
*/

func stringAppend(_a, _b Any) Any {
	a, ok := _a.(string)
	if !ok { return TypeError("string", _a) }
	b, ok := _b.(string)
	if !ok { return TypeError("string", _b) }
	return a + b
}

func strToVec(str Any) Any {
	s, ok := str.(string)
	if !ok { return TypeError("string", str) }
	rs := bytes.Runes([]byte(s))
	res := make(Vector, len(rs))
	for i, x := range rs { res[i] = x }
	return res
}

func objToStr(obj Any) Any {
	return toWrite("%v", obj)
}

/*
	Pairs
*/

func setCar(x, v Any) Any {
	return pairFunc(x, func(p *Pair) Any { 
		p.a = v
		return nil
	})
}

func setCdr(x, v Any) Any {
	return pairFunc(x, func(p *Pair) Any { 
		p.d = v
		return nil
	})
}


/*
	Vectors
*/

func makeVector(size, fill Any) Any {
	s, ok := size.(int)
	if !ok { return TypeError("vector", size) }
	res := make(Vector, s)
	for i,_ := range res {
		res[i] = fill
	}
	return res
}

func vectorLength(vec Any) Any {
	v, ok := vec.(Vector)
	if !ok { return TypeError("vector", vec) }
	return len(v)
}

func vectorRef(vec, idx Any) Any {
	v, ok := vec.(Vector)
	if !ok { return TypeError("vector", vec) }
	i, ok := idx.(int)
	if !ok { return TypeError("fixnum", idx) }
	return v.Get(i)
}

func vectorSet(vec, idx, val Any) Any {
	v, ok := vec.(Vector)
	if !ok { return TypeError("vector", vec) }
	i, ok := idx.(int)
	if !ok { return TypeError("fixnum", idx) }
	return v.Set(i, val)
}

func slice(vec, lo, hi Any) Any {
	v, ok := vec.(Vector)
	if !ok { return TypeError("vector", vec) }
	l, ok := lo.(int)
	if !ok { return TypeError("fixnum", lo) }
	h, ok := hi.(int)
	if !ok { return TypeError("fixnum", hi) }
	if l < 0 { return Error(fmt.Sprintf("invalid index (%v)", h)) }
	if h > len(v) { return Error(fmt.Sprintf("invalid index (%v)", h)) }
	return v[l:h]
}

func lsToVec(lst Any) Any {
	l := ListLen(lst)
	if l == -1 { return TypeError("pair", lst) }
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
	if !ok { return TypeError("vector", vec) }
	var res Any = EMPTY_LIST
	for i := len(xs) - 1; i >= 0; i-- {
		res = Cons(xs[i], res)
	}
	return res
}

func vecToStr(vec Any) Any {
	cs, ok := vec.(Vector)
	if !ok { return TypeError("vector", vec) }
	res := make([]int, len(cs))
	for i, c := range cs {
		r, ok := c.(int)
		if !ok { return TypeError("vector of fixnums", vec) }
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
	if !ok { return TypeError("input-port", port) }
	r, _, err := p.ReadRune()
	if err != nil {
		if err == os.EOF { return EOF_OBJECT }
		return SystemError(err)
	}
	return string(r)
}

func readByte(port Any) Any {	
	p, ok := port.(io.Reader)
	if !ok { return TypeError("input-port", port) }
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
	if !ok { return TypeError("output-port", port) }
	s, ok := str.(string)
	if !ok { return TypeError("string", str) }
	_, err := p.WriteString(s)
	if err != nil { return SystemError(err) }
	return nil
}

func writeByte(port, bte Any) Any {	
	p, ok := port.(io.Writer)
	if !ok { return TypeError("output-port", port) }
	b, ok := bte.(int)
	if !ok { return TypeError("fixnum", bte) }
	_, err := p.Write([]byte { byte(b) })
	if err != nil { return SystemError(err) }
	return nil
}

func flush(port Any) Any {
	p, ok := port.(*bufio.Writer)
	if !ok { return TypeError("output-port", port) }
	err := p.Flush()
	if err != nil { return SystemError(err) }
	return nil
}

func closePort(port Any) Any {
	p, ok := port.(io.Closer)
	if !ok { return TypeError("output-port", port) }
	err := p.Close()
	if err != nil { return SystemError(err) }
	return nil
}

