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
	_DOT peg.Terminal = iota
	_LSTART
	_LEND
	_LSTART2
	_LEND2
	_INT
	_FLOAT
	_STR
	_COMMENT
	_WS
	_QUOTE
	_SYMBOL
	_HASH
)

var lex = lexer.RegexSet {
	int(_DOT):			"\\.",
	int(_LSTART): 		"\\(",
	int(_LEND):			"\\)",
	int(_LSTART2): 		"\\[",
	int(_LEND2):		"\\]",
	int(_INT):			"\\d+",
	int(_FLOAT):		"\\d+(\\.\\d+)?",
	int(_STR):			"\"([^\"]|\\.)*\"",
	int(_COMMENT):		";[^\n]*",
	int(_WS):			"\\s+",
	int(_QUOTE):		"'|`|,|,@",
	int(_SYMBOL):		"[^#\\(\\)\"\n\r\t\\[\\].'`,@ ]+",
	int(_HASH):			"#\\a",
}

func parseList(x interface{}) interface{} {
	expr := x.([]interface{})
	ls := expr[1].([]interface{})
	var res Any = expr[2]
	for i := len(ls) - 1; i >= 0; i-- {
		x := ls[i]
		if Failed(x) { return x }
		res = Cons(x, res)
	}
	return res
}

var syntax = func() peg.Expr {
	expr := peg.Extensible()
	listEnd := peg.Bind(
		peg.Option(peg.Select(peg.And { _DOT, expr }, 1)), 
		func(x interface{}) interface{} {
			o := x.([]interface{})
			if len(o) != 0 {
				return o[0]
			}
			return EMPTY_LIST
		},
	)
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
			if err != nil { return SystemError(err) }
			return s
		}),
		peg.Bind(peg.And { _LSTART,	peg.Repeat(expr), listEnd, _LEND }, parseList),
		peg.Bind(peg.And { _LSTART2, peg.Repeat(expr), listEnd, _LEND2 }, parseList),
		peg.Bind(peg.And { _QUOTE, expr }, func(x interface{}) interface{} {
			qu := x.([]interface{})
			switch qu[0].(string) {
				case "'": return List(Symbol("quote"), qu[1])
				case "`": return List(Symbol("quasiquote"), qu[1])
				case ",": return List(Symbol("unquote"), qu[1])
			}
			return List(Symbol("unquote-splicing"), qu[1])
		}),
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
	return expr
}()

func ReadLine(port Any) (string, os.Error) {
	pt, ok := port.(io.Reader)
	if !ok { return "", TypeError(pt) }
	buf := []byte{0}
	res := ""
	for {
		_, err := pt.Read(buf)
		if err == os.EOF { return "", EOF_OBJECT.(os.Error) }
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
	l := lexer.New()
	l.Regexes(nil, lex)
	r := strings.NewReader(s)
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

