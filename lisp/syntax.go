package lisp

import (
	"io"
	"os"
	"fmt"
	"strings"
	"strconv"
	"./lexer"
	"./peg"
)

const (
	_LSTART peg.Terminal = iota
	_LEND
	_LSTART2
	_LEND2
	_INT
	_FLOAT
	_STR
	_COMMENT
	_WS
	_SYMBOL
	_HASH
)

var lex = lexer.RegexSet {
	int(_LSTART): 		"\\(",
	int(_LEND):			"\\)",
	int(_LSTART2): 		"\\[",
	int(_LEND2):		"\\]",
	int(_INT):			"\\d+",
	int(_FLOAT):		"\\d+(\\.\\d+)?",
	int(_STR):			"\"([^\"]|\\.)*\"",
	int(_SYMBOL):		"[^#\\(\\)\"\n\r\t ]+",
	int(_HASH):			"#\\a",
	int(_COMMENT):		";[^\n]*",
	int(_WS):			"\\s+",
}

func parseList(x interface{}) interface{} {
	ls := x.([]interface{})
	var res Any = EMPTY_LIST
	for i := len(ls) - 1; i >= 0; i-- {
		x := ls[i]
		if Failed(x) { return x }
		res = Cons(x, res)
	}
	return res
}

var syntax = func() peg.Expr {
	expr := peg.Extensible()
	expr.Add(peg.Or {
		peg.Bind(_INT, func(x interface{}) interface{} { 
			s, ok := x.(string)
			if !ok { return TypeError(x) }
			res, err := strconv.Atoi(s)
			if err != nil { return SystemError(err) }
			return res
		}),
		peg.Bind(_FLOAT, func(x interface{}) interface{} { 
			s, ok := x.(string)
			if !ok { return TypeError(x) }
			res, err := strconv.Atof(s)
			if err != nil { return SystemError(err) }
			return res
		}),
		peg.Bind(_STR, func(x interface{}) interface{} { 
			s, err := strconv.Unquote(x.(string))
			if err != nil { return SystemError(nil) }
			return s
		}),
		peg.Bind(peg.Select(peg.And { _LSTART, peg.Repeat(expr), _LEND }, 1), parseList),
		peg.Bind(peg.Select(peg.And { _LSTART2, peg.Repeat(expr), _LEND2 }, 1), parseList),
		peg.Bind(_SYMBOL, func(x interface{}) interface{} { return Symbol(x.(string)) }),
		peg.Bind(_HASH, func(x interface{}) interface{} {
			s := x.(string)
			switch s[1] {
				case 'v': return nil
				case 'f': return false
				case 't': return true
			}
			return SyntaxError("unknown constant: " + string(s[1]))
		}),
	})
	return expr //peg.Select(peg.And { peg.Repeat(_WS), expr, peg.Eof }, 1)
}()

func ReadLine(port Any) (string, os.Error) {
	pt, ok := port.(io.Reader)
	if !ok { return "", TypeError(pt) }
	buf := []byte{0}
	res := ""
	for {
		_, err := pt.Read(buf)
		if err != nil { return "", SystemError(err) }
		if buf[0] == '\n' { break }
		res += string(buf)
	}
	return res, nil
}

func Read(port Any) Any {
	s, err := ReadLine(port)
	if err != nil { return err }
	return ReadString(s)
}

func ReadString(s string) Any {
	r := strings.NewReader(s)
	l := lexer.New()
	l.Regexes(nil, lex)
	src := peg.NewLex(r, l, func(id int) bool { return id != int(_WS) })
	m, d := syntax.Match(src)
	if m.Failed() { return Throw(Symbol("syntax-error"), "failed to parse") }
	return d	
}

func toWrite(obj Any, def string) string {
	if obj == nil { return "#v" }
	if b, ok := obj.(bool); ok {
		if b {
			return "#t"
		} else {
			return "#f"
		}
	}
	return fmt.Sprintf(def, obj)
}

func Write(obj, port Any) Any {
	p, ok := port.(io.Writer)
	if !ok { return TypeError(port) }
	io.WriteString(p, toWrite(obj, "%#v"))
	return nil
}

func Display(obj, port Any) Any {
	p, ok := port.(io.Writer)
	if !ok { return TypeError(port) }
	io.WriteString(p, toWrite(obj, "%v"))
	return nil
}

