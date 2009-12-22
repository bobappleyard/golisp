package lisp

import ()

type compiler struct {
	syms *symbolSet;
	macros map[string] *Box;
}

func (self *compiler) compileExpr(expr Value) Value {
	return self.variables(
		EMPTY_LIST, 
		EMPTY_LIST, 
		EMPTY_LIST,
		self.closures(EMPTY_LIST, self.Expand(expr))
	);
}

func (self *compiler) getMacro(expr Value) (Value, bool) {
	if p, ok := expr.(*Pair); ok {
		var s *Symbol;
		if s, ok = p.a.(*Symbol); ok {
			var m Value;
			if m, ok = self.macros[s.name]; ok {
				return m, ok;
			}
		}
	}
	return nil, false;
}

//
// If the expression is a macro form, expand it once.
//
func (self *Interpreter) PartialExpand(expr Value) (Value, bool) {
	if m, ok := self.getMacro(expr); ok {
		return self.Apply(m, ListToVector(Cdr(expr)));
	}
	return expr, false;
}

//
// Transform a Golisp expression from one with macro forms to one without.
//
func (self *Interpreter) Expand(expr Value) Value {
	ex := func(x Value) Value {
		return self.Expand(x);
	};
	ss := self.compiler.syms;
	if p, ok := expr.(*Pair); ok {
		switch p.a {
			case ss._quote:
				return expr;
			case ss._if, ss._set:
				return Cons(p.a, Map(ex, p.d));
			case ss._lambda:
				return Append(
					List([]Value { ss.Lambda, ListRef(expr, 1) }),
					Map(ex, ListTail(expr, 2))
				);
			default:
				r, ch := self.PartialExpand(expr);
				if ch {
					return ex(r);
				}
				return Map(ex, expr);
		}
	}
	return expr;
}

func (self *compiler) scanFree(inScope, expr Value) Value {
	sf := func(ls Value) Value {
		return Fold(func(x, acc Value) Value {
			return Append(self.scanFree(inScope, x), acc);
		}, ls);
	};
	ss := self.syms;
	switch p := p.(type) {
		case *Pair:
			switch p.a {
				case ss._quote:
					return EMPTY_LIST;
				case ss._if, ss._set:
					return sf(p.d);
				case ss._lambda:
					return sf(ListTail(expr, 2));
				default:
					return sf(expr);
			}
		case *Symbol:
			if IsTrue(Member(expr, inScope)) {
				return Cons(expr, EMPTY_LIST);
			}
	}
	return EMPTY_LIST;
}

func (self *compiler) scanSet(inScope, expr Value) Value {
	sf := func(ls Value) Value {
		return Fold(func(x, acc Value) Value {
			return Append(self.scanSet(inScope, x), acc);
		}, ls);
	};
	ss := self.syms;
	if p, ok := expr.(*Pair); ok {
		switch p.a {
			case ss._quote:
				return EMPTY_LIST;
			case ss._if:
				return sf(p.d);
			case ss._lambda:
				return sf(ListTail(expr, 2));
			case ss._set:
				if IsTrue(Member(expr, inScope)) {
					return Cons(expr, EMPTY_LIST);
				}
			default:
				return sf(expr);
		}
	}
	return EMPTY_LIST;
}

func (self *compiler) closures(inScope, expr Value) Value {
	cf := func(x Value) Value {
		return Reverse(Fold(func(x, acc Value) Value {
			return Append(self.closures(inScope, x), acc);
		}, ls));
	};
	ss := self.syms;
	if p, ok := expr.(*Pair); ok {
		switch p.a {
			case ss._quote:
				return expr;
			case ss._if, ss._set:
				return Cons(p.a, cf(p.d));
			case ss._lambda:
				hd := List([]Value {
					ss._lambda,
					self.scanFree(inScope, expr),
					self.scanSet(ListRef(expr, 1), expr)
				});
				inScope = Append(ListRef(expr, 1), inScope);
				return Append(hd, cf(ListTail(expr, 2)));
			default:
				return cf(expr);
		}
	}
	return expr;
}

func (self *compiler) variables(bound, free, boxed, expr Value) Value {
	sf := func(ls Value) Value {
		return Fold(func(x, acc Value) Value {
			return Append(self.scanFree(inScope, x), acc);
		}, ls);
	};
	ss := self.syms;
	switch p := p.(type) {
		case *Pair:
			switch p.a {
				case ss._quote:
					return EMPTY_LIST;
				case ss._if, ss._set:
					return sf(p.d);
				case ss._lambda:
					return sf(ListTail(expr, 2));
				default:
					return sf(expr);
			}
		case *Symbol:
			if IsTrue(Member(expr, inScope)) {
				return Cons(expr, EMPTY_LIST);
			}
	}
	return expr;
}

