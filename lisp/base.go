package lisp

import (
	v "container/vector";
)

type (
	// Name for dynamic typery
	Value interface{};
	// A primitive procedure
	Primitive func(args []Value) Value;
)

func ParseArgs(src []Value, dst []*Value) {
	for i, v := range dst {
		*v = src[i];
	}
}

