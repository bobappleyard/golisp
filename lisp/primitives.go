package lisp

import (
	"big"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
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
		"load": load,
		"eval": eval,
		"apply": apply,
		"throw": throw,
		"catch": catch,
		"null-environment": nullEnv,
		"capture-environment": capEnv,
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
		"fixnum-quotient": quotient,
		"fixnum-modulo": modulo,
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
		"close-port": closePort,
	})
}

/*
	Equality
*/

func eq(a, b interface{}) interface{} {
	return a == b
}

/*
	Syntax
*/

func readStr(str interface{}) interface{} {
	s, ok := str.(string)
	if !ok { return TypeError("string", str) }
	return ReadString(s)
}

func newMacro(m interface{}) interface{} {
	f, ok := m.(Function)
	if !ok { return TypeError("function", m) }
	return &macro { f }
}

/* 
	Control
*/

func load(path, env interface{}) interface{} {
	ctx, ok := env.(*Scope)
	if !ok { return TypeError("environment", env) }
	p, ok := path.(string)
	if !ok { return TypeError("string", path) }
	return ctx.Load(p)
}

func eval(expr, env interface{}) interface{} {
	ctx, ok := env.(*Scope)
	if !ok { return TypeError("environment", env) }
	return ctx.Eval(expr)
}

func apply(f, args interface{}) interface{} {
	fn, ok := f.(Function)
	if !ok { return TypeError("function", f) }
	return fn.Apply(args)
}

func throw(kind, msg interface{}) interface{} {
	k, ok := kind.(Symbol)
	if !ok { return TypeError("symbol", kind) }
	return Throw(k, msg)
}
		
func catch(thk, hnd interface{}) interface{} {
	t, ok := thk.(Function)
	if !ok { return TypeError("function", t) }
	h, ok := hnd.(Function)
	if !ok { return TypeError("function", h) }
	res := Call(t)
	// handle interface{} errors
	if r, ok := res.(*errorStruct); ok {
		res = Call(h, r.kind, r.msg)
	}
	return res
}

func nullEnv() interface{} {
	return NewScope(nil)
}

func capEnv(env interface{}) interface{} {
	e, ok := env.(*Scope)
	if !ok { return TypeError("environment", env) }
	return NewScope(e)
}

/*
	Type system
*/

func typeOf(x interface{}) interface{} {
	s := "unknown"
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
		case *big.Int: s = "bignum"
	}
	if x == nil { s = "void" }
	return Symbol(s)
}

func newCustom(name, fn interface{}) interface{} {
	n, ok := name.(Symbol)
	if !ok { return TypeError("symbol", name) }
	f, ok := fn.(Function)
	if !ok { return TypeError("function", fn) }
	wrap := WrapPrimitive(func(x interface{}) interface{} { 
		return NewCustom(n, x) 
	})
	unwrap := WrapPrimitive(func(x interface{}) interface{} {
		c, ok := x.(*Custom)
		if !ok || c.name != n { return TypeError(string(n), x) }
		return c.Get()
	})
	set := WrapPrimitive(func(x, v interface{}) interface{} {
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

func symToStr(sym interface{}) interface{} {
	s, ok := sym.(Symbol)
	if !ok { return TypeError("symbol", sym) }
	return string(s)
}

func strToSym(str interface{}) interface{} {
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

func gensym() interface{} {
	return <-gensyms
}

/*
	Numbers
*/

func fixToFlo(_x interface{}) interface{} {
	switch x := _x.(type) {
		case int: return float(x)
		case float: return x
	}
	return TypeError("number", _x) 
}

func fixnumFunc(_a, _b interface{}, f func (a, b int) interface{}) interface{} {
	a, ok := _a.(int)
	if !ok { return TypeError("fixnum", _a) }
	b, ok := _b.(int)
	if !ok { return TypeError("fixnum", _b) }
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
		if b == 0 { return Error("divide by zero") }
		if a % b == 0 { return a / b }
		return float(a) / float(b)
	})
}

func quotient(_a, _b interface{}) interface{} {
	return fixnumFunc(_a, _b, func(a, b int) interface{} {
		if b == 0 { return Error("divide by zero") }	
		return a / b
	})
}

func modulo(_a, _b interface{}) interface{} {
	return fixnumFunc(_a, _b, func(a, b int) interface{} {
		if b == 0 { return Error("divide by zero") }
		return a % b
	})
}

func flonumFunc(_a, _b interface{}, f func(a, b float) interface{}) interface{} {
	a, ok := _a.(float)
	if !ok { return TypeError("flonum", _a) }
	b, ok := _b.(float)
	if !ok { return TypeError("flonum", _b) }
	return f(a, b)
}

func flonumAdd(_a, _b interface{}) interface{} {
	return flonumFunc(_a, _b, func(a, b float) interface{} { return a + b })
}

func flonumSub(_a, _b interface{}) interface{} {
	return flonumFunc(_a, _b, func(a, b float) interface{} { return a - b })
}

func flonumMul(_a, _b interface{}) interface{} {
	return flonumFunc(_a, _b, func(a, b float) interface{} { return a * b })
}

func flonumDiv(_a, _b interface{}) interface{} {
	return flonumFunc(_a, _b, func(a, b float) interface{} {
		if b == 0 { return Error("divide by zero") }
		return a / b
	})
}

/*
	Strings
*/

func stringSplit(str, sep interface{}) interface{} {
	s, ok := str.(string)
	if !ok { return TypeError("string", str) }
	b, ok := sep.(string)
	if !ok { return TypeError("string", sep) }
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
	if !ok { return TypeError("string", sep) }
	for cur, i := strs, 0; cur != EMPTY_LIST; cur, i = Cdr(cur), i + 1 {
		x := Car(cur)
		s, ok := x.(string)
		if !ok { return TypeError("string", x) }
		ss[i] = s
	}
	return strings.Join(ss, b)
}

func strToVec(str interface{}) interface{} {
	s, ok := str.(string)
	if !ok { return TypeError("string", str) }
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
	if !ok { return TypeError("vector", size) }
	res := make(Vector, s)
	for i,_ := range res {
		res[i] = fill
	}
	return res
}

func vectorLength(vec interface{}) interface{} {
	v, ok := vec.(Vector)
	if !ok { return TypeError("vector", vec) }
	return len(v)
}

func vectorRef(vec, idx interface{}) interface{} {
	v, ok := vec.(Vector)
	if !ok { return TypeError("vector", vec) }
	i, ok := idx.(int)
	if !ok { return TypeError("fixnum", idx) }
	return v.Get(i)
}

func vectorSet(vec, idx, val interface{}) interface{} {
	v, ok := vec.(Vector)
	if !ok { return TypeError("vector", vec) }
	i, ok := idx.(int)
	if !ok { return TypeError("fixnum", idx) }
	return v.Set(i, val)
}

func slice(vec, lo, hi interface{}) interface{} {
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

func lsToVec(lst interface{}) interface{} {
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

func vecToLs(vec interface{}) interface{} {
	xs, ok := vec.(Vector)
	if !ok { return TypeError("vector", vec) }
	var res interface{} = EMPTY_LIST
	for i := len(xs) - 1; i >= 0; i-- {
		res = Cons(xs[i], res)
	}
	return res
}

func vecToStr(vec interface{}) interface{} {
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

func openFile(path, mode interface{}) interface{} {
	p, ok := path.(string)
	if !ok { return TypeError("string", path) }
	m, ok := mode.(Symbol)
	if !ok { return TypeError("symbol", mode) }
	wrap := func(x interface{}) interface{} { return bufio.NewWriter(x.(io.Writer)) }
	filemode, perms := 0, 0
	switch string(m) {
		case "create": filemode, perms = os.O_CREAT, 0644
		case "read": filemode, wrap = os.O_RDONLY, func(x interface{}) interface{} {
			return bufio.NewReader(x.(io.Reader))
		}
		case "write": filemode = os.O_WRONLY
		case "append": filemode = os.O_APPEND
		default: return Error(fmt.Sprintf("wrong access token: %s", m))
	}
	f, err := os.Open(p, filemode, perms)
	if err != nil { return SystemError(err) }
	return wrap(f)
}

func readChar(port interface{}) interface{} {
	p, ok := port.(*bufio.Reader)
	if !ok { return TypeError("input-port", port) }
	r, _, err := p.ReadRune()
	if err != nil {
		if err == os.EOF { return EOF_OBJECT }
		return SystemError(err)
	}
	return string(r)
}

func readByte(port interface{}) interface{} {	
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

func isEof(x interface{}) interface{} {
	return x == EOF_OBJECT
}

func writeString(port, str interface{}) interface{} {
	p, ok := port.(*bufio.Writer)
	if !ok { return TypeError("output-port", port) }
	s, ok := str.(string)
	if !ok { return TypeError("string", str) }
	_, err := p.WriteString(s)
	if err != nil { return SystemError(err) }
	return nil
}

func writeByte(port, bte interface{}) interface{} {	
	p, ok := port.(io.Writer)
	if !ok { return TypeError("output-port", port) }
	b, ok := bte.(int)
	if !ok { return TypeError("fixnum", bte) }
	_, err := p.Write([]byte { byte(b) })
	if err != nil { return SystemError(err) }
	return nil
}

func flush(port interface{}) interface{} {
	p, ok := port.(*bufio.Writer)
	if !ok { return TypeError("output-port", port) }
	err := p.Flush()
	if err != nil { return SystemError(err) }
	return nil
}

func closePort(port interface{}) interface{} {
	p, ok := port.(io.Closer)
	if !ok { return TypeError("output-port", port) }
	err := p.Close()
	if err != nil { return SystemError(err) }
	return nil
}

