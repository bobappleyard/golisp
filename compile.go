package lisp

import ()

type compile struct {
	syms *symbolSet;
	macros map[string] *Box;
}

func (self *compile) Compile(expr Value) Value {
	return self.variables(self.closures(self.Expand(expr)));
}

func (self *compile) getMacro(expr Value) (Value, bool) {
	if p, ok := expr.(*Pair); ok {
		var s string;
		if s, ok = p.a.(*Symbol); ok {
			var m Value;
			if m, ok = self.macros[s.name]; ok {
				return m, ok;
			}
		}
	}
	return nil, false;
}

func (self *Interpreter) PartialExpand(expr Value) (Value, bool) {
	if m, ok := self.getMacro(expr); ok {
		return self.Apply(m, ListToVector(Cdr(expr)));
	}
	return expr, false;
}

func (self *Interpreter) Expand(expr Value) Value {
	if p, ok := expr.(*Pair); ok {
		ss := self.syms;
		switch p.a {
			case ss._quote:
				return expr;
			case ss._if:
				return List(
					ss._if, 
					self.Expand(ListRef(expr, 1)),
					self.Expand(ListRef(expr, 2)),
					self.Expand(ListRef(expr, 3))
				);
			case ss._lambda:
				return Cons(
					ss._lambda,
					Cons(
						ListRef(expr, 1),
						Map(self.Expand, ListTail(expr, 2))
					)
				);
			case ss._set:
				return List(
					ss._set,
					self.Expand(ListRef(expr, 1)),
					self.Expand(ListRef(expr, 2))
				);
			default:
				r, ch := self.PartialExpand(expr);
				if ch {
					return self.Expand(r);
				}
				return Map(self.Expand, expr);
		}
	}
	return expr;
}

func appendSyms(ss [][]*Symbol) []*Symbol {
	var l int;
	for _, x := range ss {
		l += len(x);
	}
	res := make([]*Symbol, l);
	var i int;
	for _, ls := range ss {
		for _, x := range ls {
			res[i] = x;
			i++;
		}
	}
	return res;
}

func (self *compile) appendSyms(ls Value) []*Symbol {

}

func (self *compile) scanFree(inScope, expr Value) []*Symbol {
	empty := new([]*Symbol);
	ss := self.syms;
	switch p := p.(type) {
		case *Pair:
			switch p.a {
				case ss._quote:
					return empty;
				case ss._lambda:
					return Append(self.scanFree(inScope, ListTail(expr, 2)), inScope);
				case ss._if, ss._set:
					return self.appendSyms(ListTail(expr, 1));
				default:
					return self.appendSyms(expr);
			}
		case *Symbol:
	}
	return empty;
}

func (self *compile) scanSet(expr Value) []*Symbol {

}

func (self *compile) closures(expr Value) Value {
	return List(
		self.syms._lambda,
		scanFree(expr),
		scanSet(expr),
	);
}

func (self *compile) variables(expr Value) Value {
	return expr;
}

