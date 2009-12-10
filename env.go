package lisp

import (
	"strings";
	v "container/vector";
)

type Symbol struct {
	name string;
	val *Box; // where global variables are stored
}

/* Methods on Symbol */

func (self *Symbol) Name() string {
	return self.name;
}

type (
	pkg struct {
		name, desc string;
		exports *v.StringVector;
		mappings map[string] *Symbol;
	};
	environment struct {
	/* The package set */
		current *pkg;
		packages map[string] *pkg;
	/* The symbol table */
		symbols map[string] *Symbol;		
	};
)

/* Access the symbol table */

func (self *environment) Symbol(name string) *Symbol {
	// gensym
	if name == "" {
		return new(Symbol);
	}
	// look in the current package
	if strings.Index(name, ":") == -1 {
		res, ok := self.current.mappings[name];
		if ok {
			return res;
		}
		if self.current.name != "" {
			name = self.current.name + ":" + name;
		}
	}
	// look in the symbol table
	res, ok := self.symbols[name];
	if ok {
		return res;
	}
	// make a new symbol (and add it to the table)
	res = &Symbol { name, NewBox(UndefinedError(name)) };
	self.symbols[name] = res;
	return res;
}

func (self *environment) AddGlobal(name string, val Value) {
	self.Symbol(name).Rebox(val);
}

/* Manipulating the package set */

func (self *environment) ListPackages() (names, descs []string) {
	names = make([]string, len(self.packages));
	descs = make([]string, len(self.packages));
	i := 0;
	for k, v := range self.packages {
		names[i] = k;
		descs[i] = v.desc;
		i++;
	}
	return;
}

func (self *environment) PackageExists(name string) bool {
	_, ok := self.packages[name];
	return ok;
}

func (self *environment) SetPackage(name string) {
	p, ok := self.packages[name];
	if !ok {
		p = &pkg { 
			name, 
			"",
			new(v.StringVector), 
			make(map[string] *Symbol)
		};
		self.packages[name] = p;
	}
	self.current = p;
}

func (self *environment) PackageName() string {
	return self.current.name;
}

func (self *environment) PackageDescription() string {
	return self.current.desc;
}

func (self *environment) SetPackageDescription(d string) {
	self.current.desc = d;
}	

/* Control variable sharing */

func (self *environment) Export(name string) {
	self.current.exports.Push(name);
}

func (self *environment) Import(pkg string) {
	p := self.current;
	old := p.name;
	self.Setpkg(pkg);
	d := self.current.exports;
	for export := range d.Iter() {
		p.mappings[export] = self.Symbol(export);
	}
	self.Setpkg(old);
}

/* Initialisation */

func (self *environment) initEnvironment() {
	self.packages = make(map[string] *pkg);
	self.symbols = make(map[string] *Symbol);
	self.Setpkg("");
}

type symbolSet struct {
	_quote, _if, _lambda, _set,
	_bound, _free, _global, _unbox *Symbol;
}

func (self *exec) initSyms() *symbolSet {
	return &symbolSet {
		self.Symbol("_quote"), self.Symbol("_if"), self.Symbol("_lambda")
		self.Symbol("_set"), self.Symbol("_bound"), self.Symbol("_free"), 
		self.Symbol("_global"), self.Symbol("_unbox")
	}
}


