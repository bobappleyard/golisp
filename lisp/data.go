package lisp

import (
	"big";
)

/*
	Atomic types
*/

func IsBool(x Value) (res bool) {
	_, res = x.(bool);
	return;
}

// anything other than false is true
func IsTrue(x Value) bool {
	b, ok := x.(bool);
	return b || !ok;
}

type Number interface {
	Int() Fixnum;
	Float() Flonum;
}

// yer basic number tower
type ( 
	Fixnum int;
	Bignum big.Int;
	Rational struct { n, d Int; };
	Flonum float;
	Complex struct { r, i Float; };
)


/*
    Pairs
*/

type Pair struct { 
    a, d LispObject; 
}

func Cons(a, d Value) Value {
    return &Pair { a, d };
}

func IsPair(x Value) bool {
	_, ok := x.(*Pair);
	return ok;
}

func pairFunc(f func(*Pair) Value, p Value) Value {
	if IsPair(p) {
		return f(p);
    }
	return TypeError(x);
}

func Car(x Value) Value {
	return pairFunc(func(p *Pair) Value {
        return p.a;
	}, x);
}

func Cdr(x Value) Value {
	return pairFunc(func(p *Pair) Value {
        return p.d;
	}, x);
}

func SetCar(x, v Value) Value {
	return pairFunc(func(p *Pair) Value {
		p.a = v;
        return VOID;
	}, x);
}

func SetCdr(x, v Value) Value {
	return pairFunc(func(p *Pair) Value {
		p.d = v;
        return VOID;
	}, x);
}

/*
	List stuff
*/

func ListTail(x Value, i int) Value {
	for ; i > 0; i-- {
		n := Cdr(x);
		if Failed(n) {
			return n;
		}
		x = n;
	}
	return x;
}

func ListRef(x Value, i int) Value {
	t := ListTail(x, i));
	if Failed(t) {
		return t;
	}
	return Car(t);
}

func List(vs []Value) Value {
	res := EMPTY_LIST;
	for i := len(vs) - 1; i >= 0; i-- {
		res = Cons(vs[i], res);
	}
	return res;
}

func Fold(f func(x, acc Value) Value, acc, ls Value) Value {
	for ; ls != EMPTY_LIST; ls = Cdr(ls) {
		if Failed(ls) {
			return ls;
		}
		x := Car(ls);
		if Failed(x) {
			return x;
		}
		acc = f(x, acc);
	}
	return acc;
}

func Reverse(ls Value) Value {
	return Fold(Cons, EMPTY_LIST, ls);
}

func Map(f func(x Value) Value, ls Value) Value {
	return Reverse(Fold(func(x, acc Value) Value {
		return Cons(f(x), acc);
	}, EMPTY_LIST, ls));
}

func Length(ls Value) (i int) {	
	for i = 0; ls != EMPTY_LIST; i, ls = i + 1, Cdr(ls) {}
	return;
}




