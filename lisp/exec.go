package lisp

import (
	"os";
)

type Box struct {
	Unbox() Value;
	Rebox(val Value);
}

func NewBox(val Value) *Box {
	res := new(localBox);
	res.val = val;
	return res;
}

func (self *Box) Unbox() Value {
	return self.val;
}

func (self *Box) Rebox(val Value) {
	self.val = val;
}

type Closure struct {
	env []Value;
	boxed, vars, body Value;
	argc int;
	vargs bool;
}

func (self *Closure) String() string {
	return "#<lambda " + ToString(self.vars) + ">";
}

func (self *Closure) init() {
	// a little more preprocessing to help the interpreter out
	x := self.vars;
	i := 0;
	for {
		if x == EMPTY_LIST {
			self.argc = i;
			self.vargs = false;
			return;
		}
		if p, ok := x.(*Pair); ok {
			i++;
			x = p.d;
			continue;
		}
		self.argc = i;
		self.vargs = true;
		return;
	}
}

type (
	tailStruct struct {
		proc *Value;
		args *[]Value;
	};
	exec struct {
		syms *symbolSet;
	};
)

func (self *Interpreter) Eval(expr Value) Value {
	blank := new([]Value);
	return self.evalExpr(self.Compile(expr), blank, blank, nil);
}

func (self *Interpreter) EvalString(expr string) Value {
	return self.Eval(self.ReadString(expr));
}

func (self *exec) Apply(f Value, args []Value) (value Value) {
	// Yer basic trampoline.
	// Stops when f == nil after the switch statement.
	// Only the tail call can modify f.
	tail := &tailStruct { &f, &args };
	for {
		switch p := f.(type) {
			case Primitive:
				// so simple when it's a primitive
				f = nil;
				value = p(args);
			case *Closure:
				f = nil;
				// function prologue
				l := len(args);
				if p.vargs {
					// varargs support
					if l < p.argc {
						return ArgumentError(args);
					}
					switch l {
						case 0:
							args = []Value { EMPTY_LIST };
						case p.argc:
							new_args := make([]Value, l + 1);
							for i, x := range args {
								new_args[i] = x;
							}
							new_args[l] = EMPTY_LIST;
							args = new_args;
						default:
							args[p.argc] = List(args[p.argc:l]);
							args = args[0:p.argc];
					}
				}
				if !p.vargs && l != p.argc {
					return ArgumentError(args);
				}
				// box up any variables that need it
				for x := p.boxed; x != EMPTY_LIST; x = Cdr(x) {
					n := Car(x).(Int)
					args[n] = newBox(args[n]);
				}
				// eval the body
				env := p.env;
				value = VOID;
				for x := p.body; x != EMPTY_LIST; x = Cdr(x) {
					e := Car(x);
					if Cdr(x) == EMPTY_LIST {
						// in tail position (this can keep the call going)
						value = self.evalExpr(e, args, env, tail);
					} else {
						// not in tail position
						value = self.evalExpr(e, args, env, nil);
					}
					// an error cancels the call
					if Failed(value) {
						return;
					}
				}
			default:
				return TypeError(proc);
		}
		// stop if the tail call didn't result in more things to call
		if f == nil {
			break;
		}
	}
	return;
}

func (self *exec) evalExpr(expr Value, args, env []Value, tail *tailStruct) Value {
	// where the interpreter really lives
	if e, ok := expr.(*Pair); ok {
		ss := self.syms;
		switch Car(e) {
			// standard forms
			case ss._quote:
				return ListRef(expr, 1);
			case ss._if:
				test := self.evalExpr(ListRef(expr, 1)), args, env, nil);
				if Failed(test) {
					return test;
				}								
				if IsTrue(test) {
					return self.evalExpr(ListRef(expr, 2), args, env, tail);
				} else {
					return self.evalExpr(ListRef(expr, 3), args, env tail);
				}
			case ss._lambda:
				return self.evalClosure(expr, args, env);
			case ss._set:
				val := self.evalExpr(ListRef(expr, 2), args, env, nil);
				if Failed(val) {
					return val;
				}				
				box := self.evalExpr(ListRef(expr, 1), args, env, nil).(*Box)
				if x := box.Unbox(); Failed(x) {
					// stop on undefined global variable
					return x;
				}
				box.Rebox(val);
				return VOID;
			// support forms (added by the compiler)
			case ss._bound:
				return args[ListRef(expr, 1).(Int)];
			case ss._free:
				return env[ListRef(expr, 1).(Int)];
			case ss._global:
				return ListRef(expr, 1).(*Symbol).val;
			case ss._unbox:
				box := self.evalExpr(ListRef(expr, 1), args, env, nil).(*Box);
				return box.Unbox();
			// everything else is a function call
			default:
				return self.evalCall(expr, args, env, tail);
		}
	}
	// if it isn't a list it's some value or other
	return expr;
}

func (self *exec) evalClosure(expr Value, args, env []Value) Value {
	// create a closed environment for the function
	closed := ListRef(expr, 1);
	new_env := make([]Value, Length(closed));
	for i, x := 0, closed; x != EMPTY_LIST; i, x = i + 1, Cdr(x) {
		// global variables don't get added to closures, so no error checking required
		new_env[i] = self.evalExpr(x, args, env, nil);
	}
	// create the function
	f := new(Closure);
	f.env = new_env;
	f.boxed = ListRef(expr, 2);
	f.vars = ListRef(expr, 3);
	f.body = ListTail(expr, 4);
	f.init();
	return f;
}

func (self *exec) evalCall(expr Value, args, env []Value, tail *tailStruct) Value {
	// eval the procedure
	proc := self.evalExpr(Car(expr), args, env, nil);
	if Failed(proc) {
		return proc;
	}
	// eval the arguments
	new_args := make([]Value, Length(expr) - 1);
	for i, x := 0, Cdr(expr); x != EMPTY_LIST; i, x = i + 1, Cdr(x) {
		e := self.evalExpr(Car(x), args, env, nil);
		if Failed(e) {
			return e;
		}
		new_args[i] = e;
	}
	// feed back to the trampoline,l if present
	if tail != nil {
		*(tail.proc) = proc;
		*(tail.args) = new_args;
		return nil;
	}
	// otherwise create a new binding frame
	return self.Apply(proc, new_args);
}

func (self *exec) initExec(s *symbolSet) {
	self.syms = s;
}

