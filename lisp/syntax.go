package lisp

import (
	"big"
	"io"
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
	_VSTART
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
	int(_VSTART):		"#\\(",
	int(_INT):			"-?\\d+",
	int(_FLOAT):		"-?\\d+(\\.\\d+)?",
	int(_STR):			"\"([^\"]|\\.)*\"",
	int(_COMMENT):		";[^\n]*",
	int(_WS):			"\\s+",
	int(_QUOTE):		"'|`|,|,@",
	int(_SYMBOL):		"[^#\\(\\)\"\n\r\t\\[\\]'`,@ ]+",
	int(_HASH):			"#.",
}

func listExpr(start, rec, end peg.Expr) peg.Expr {
	tail := peg.Bind(
		peg.Option(peg.Select(peg.And { _DOT, rec }, 1)),
		func(x interface{}) interface{} {
			o := x.([]interface{})
			if len(o) != 0 {
				return o[0]
			}
			return EMPTY_LIST
		},
	)
	inner := peg.Bind(
		peg.Option(peg.And { peg.Multi(rec), tail }),
		func(x interface{}) interface{} {
			o := x.([]interface{})
			if len(o) == 0 {
				return EMPTY_LIST
			}
			expr := o[0].([]interface{})
			ls := expr[0].([]interface{})
			res := expr[1]
			if Failed(res) { return res }
			for i := len(ls) - 1; i >= 0; i-- {
				x := ls[i]
				if Failed(x) { return x }
				res = Cons(x, res)
			}
			return res
		},
	)
	return peg.Select(peg.And { start, inner, end }, 1)
}

var syntax = func() *peg.ExtensibleExpr {
	expr := peg.Extensible()
	expr.Add(peg.Or {
		peg.Bind(_INT, func(x interface{}) interface{} { 
			s := x.(string)
			res, err := strconv.Atoi(s)
			if err != nil { 
				num := big.NewInt(0)
				_, ok := num.SetString(s, 10)
				if !ok { return SystemError(err) }
				return num
			}
			return res
		}),
		peg.Bind(_FLOAT, func(x interface{}) interface{} { 
			res, err := strconv.Atof(x.(string))
			if err != nil { return SystemError(err) }
			return res
		}),
		peg.Bind(_STR, func(x interface{}) interface{} { 
			res, err := strconv.Unquote(x.(string))
			if err != nil { return SystemError(err) }
			return res
		}),
		listExpr(_LSTART, expr, _LEND),
		listExpr(_LSTART2, expr, _LEND2),
		peg.Bind(	
			peg.Select(peg.And { _VSTART, peg.Repeat(expr), _LEND }, 1),
			func(x interface{}) interface{} {
				return Vector(x.([]interface{}))
			},
		),
		peg.Bind(peg.And { _QUOTE, expr }, func(x interface{}) interface{} {
			qu := x.([]interface{})
			s := ""
			switch qu[0].(string) {
				case "'": s = "quote"
				case "`": s = "quasiquote"
				case ",": s = "unquote"
				case ",@": s = "unquote-splicing"
			}
			return List(Symbol(s), qu[1])
		}),
		peg.Bind(_SYMBOL, func(x interface{}) interface{} {
			return Symbol(x.(string))
		}),
		peg.Bind(_HASH, func(x interface{}) interface{} {
			s := x.(string)
			switch s[1] {
				case 'v': return nil
				case 'f': return false
				case 't': return true
			}
			return SyntaxError("unknown hash syntax: " + s)
		}),
	})
	return expr
}()

func readExpr(expr peg.Expr, port interface{}) interface{} {
	p, ok := port.(*InputPort)
	if !ok { return TypeError("input-port", port) }
	if p.Eof() { return EOF_OBJECT }
	l := lexer.New()
	l.Regexes(nil, lex)
	src := peg.NewLex(p, l, func(id int) bool { 
		return id != int(_WS) && id != int(_COMMENT) 
	})
	m, d := expr.Match(src)
	if m.Failed() { 
		return Throw(
			Symbol("syntax-error"), 
			fmt.Sprintf("failed to parse (%d)", m.Pos()),
		)
	}
	return d
}

func Read(port interface{}) interface{} {
	return readExpr(
		peg.Or { 
			syntax, 
			peg.Bind(peg.Eof, func(x interface{}) interface{} { return EOF_OBJECT }),
		},
		port,
	)
}

func ReadFile(port interface{}) interface{} {
	return readExpr(
		peg.Select(peg.And {
			peg.Bind(peg.Repeat(syntax), func(x interface{}) interface{} {
				return vecToLs(Vector(x.([]interface{})))
			}),
			peg.Eof,
		}, 0),
		port,
	)
}

func ReadString(s string) interface{} {
	return Read(NewInput(strings.NewReader(s)))
}

func toWrite(def string, obj interface{}) string {
	if obj == nil { return "#v" }
	switch x := obj.(type) {
		case bool: if x {
			return "#t"
		} else {
			return "#f"
		}
		case *big.Int: return x.String()
		case *InputPort: return "#<input-port>"
		case *OutputPort: return "#<output-port>"
	}
	return fmt.Sprintf(def, obj)
}

func Write(obj, port interface{}) interface{} {
	p, ok := port.(io.Writer)
	if !ok { return TypeError("output-port", port) }
	io.WriteString(p, toWrite("%#v", obj))
	return nil
}

func Display(obj, port interface{}) interface{} {
	p, ok := port.(io.Writer)
	if !ok { return TypeError("output-port", port) }
	io.WriteString(p, toWrite("%v", obj))
	return nil
}

