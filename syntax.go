package lisp

import (
	. "peg";
)

type syntax struct {
	host *Interpreter;
	expr, hash *Extensible;
	root, ignore Expr;
}

func (self *syntax) Read(in Value) Value {
	i, ok := in.(Input);
	if ok {
		pos, val := Parse(self.root, i);
		if pos.Failed() {
			return Error("failed to parse");
		}
		return val;
	}
	return TypeError(in);
}

func (self *syntax) AddReadSyntax(e Expr) {
	self.expr.Add(e);
}

func (self *syntax) AddHashSyntax(e Expr) {
	self.hash.Add(e);
}

func (self *syntax) s_root() {
	// ignore a load of stuff first
	self.ignore = Or {
		And { Char(';'), RepeatUntil(Any, LineEnd), LineEnd },
		Whitespace
	};
	// if all else fails to parse, have a go as a symbol
	sym := Bind(Merge(Multi(Select(And {
		Prevent(Or { 
			Char(')'), 
			Char(']'),
			Whitespace, 
			EOF 
		}),
		Any
	}, 1))), func(v Data) Data {
		s := v.(string);
		if s[len(s) -1] == ':' {
			return self.host.Keyword(s);
		}
		return self.host.Symbol(s);
	});
	// setup the root to take all this into account
	self.root = Select(And {
		Repeat(self.ignore),
		Or {
			self.expr,
			sym,
			Bind(EOF, func(v Data) Data { return EOF_OBJECT; })
		}
	}, 1);
}

func (self *syntax) s_hash() {
	self.hash = NewExtensible();
	self.AddReadSyntax(Select(And {
		Char('#'),
		self.hash
	}, 1));	
}

func (self *syntax) s_atoms() {
	// characters
	self.AddHashSyntax(Bind(And { Char(`\`), Any } func(v Data) Data {
		return self.host.Char(v.([]Data)[1].(int));
	});
	// strings
	qu := Char('"');
	self.AddReadSyntax(Select(And {
		qu,
		Merge(Repeat(Or { 
			Select(And { Char('\\'), Any }, 1),
			RepeatUntil(Any, qu)
		})),
		qu
	}, 1));
	// numbers
	digits := Merge(Multi(Digit));
	self.AddReadSyntax(Bind(And {
		digits,
		Option(And {
			Or { Char('.'), Char('/') },
			digits
		})
	}, func(v Data) Data {
		vs := v.([]Data);
		n := vs[0].(string);
		fp := vs[1].([]Data);
		if len(fp) != 0 {
			fp = fp[0].([]Data);
			d := fp[1].(string);
			if fp[0].(string) == "." {
				return self.ParseNumber(n + "." + d);
			}
			return self.Rational(self.ParseNumber(n), self.ParseNumber(d));
		}
		return self.ParseNumber(n);
	}));
}

func (self *syntax) s_colls() {
	// lists
	list := Bind(And {
		Repeat(self.root),
		Option(And {
			self.root,
			Multi(self.ignore),
			Char('.'),
			self.ignore,
			self.root
		}),
	}, func(v Data) Data {
		vs := v.([]Data);
		is := vs[0].([]Data);
		tl := vs[1].([]Data);
		var p Value = EMPTY_LIST;
		if len(tl) != 0 {
			tl = tl[0].([]Data);
			p = self.Cons(tl[0], tl[3]);
		}
		for i := len(is); i >= 0; i-- {
			p = self.Cons(is[i], p);
		}
		return p;
	});
	self.AddReadSyntax(Select(Or {
		And { Char('('), list, Char(')') },
		And { Char('['), list, Char(']') } 
	}, 1));
	// vectors
	self.AddHashSyntax(Select(And { Char('('), Repeat(self.root), Char(')') }, 1));
}


//~ func (self *syntax) s_root() {


func (self *syntax) initSyntax(h *Interpreter) {
	self.expr = NewExtensible();
	self.host = h;
	self.s_root();
	self.s_hash();
	self.s_atoms();
	self.s_colls();
}



