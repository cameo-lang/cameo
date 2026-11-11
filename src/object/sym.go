package object

import "sync"

// Sym is a name as a number: the name of a variable, a method or an
// operator, interned once per process. Two names are the same name when
// their Syms are equal, so code that runs often compares and switches on
// numbers and not on strings.
//
// A Sym is not written anywhere: a unit and a dump carry names as
// strings, and each process numbers them as it meets them. The names in
// wellKnown have the same number in every process, which is what lets Go
// code switch on them as constants.
type Sym uint32

// The names with a fixed number. SymNone is no name.
const (
	SymNone Sym = iota
	SymAdd
	SymSub
	SymMul
	SymDiv
	SymMod
	SymLt
	SymLe
	SymGt
	SymGe
	SymEq
	SymNe
	SymCmp
	SymCaseEq
	SymIndex
	SymIndexSet
	SymShl
	SymNot
	SymCall
	SymParens
	SymYield
	SymEach
	SymNew
	SymInitialize
	SymToS
	SymInspect
	symWellKnown
)

var wellKnown = [symWellKnown]string{
	SymNone:       "",
	SymAdd:        "+",
	SymSub:        "-",
	SymMul:        "*",
	SymDiv:        "/",
	SymMod:        "%",
	SymLt:         "<",
	SymLe:         "<=",
	SymGt:         ">",
	SymGe:         ">=",
	SymEq:         "==",
	SymNe:         "!=",
	SymCmp:        "<=>",
	SymCaseEq:     "===",
	SymIndex:      "[]",
	SymIndexSet:   "[]=",
	SymShl:        "<<",
	SymNot:        "!",
	SymCall:       "call",
	SymParens:     "()",
	SymYield:      "yield",
	SymEach:       "each",
	SymNew:        "new",
	SymInitialize: "initialize",
	SymToS:        "to_s",
	SymInspect:    "inspect",
}

// syms is the names interned so far. A name's Sym is its index in names.
var syms = struct {
	sync.RWMutex
	byName map[string]Sym
	names  []string
}{byName: map[string]Sym{}}

func init() {
	syms.names = append(syms.names, wellKnown[:]...)
	for i, name := range wellKnown {
		syms.byName[name] = Sym(i)
	}
}

// Intern returns the Sym of name, numbering it when it is new. The name
// is kept for as long as the process runs, so Intern is for the names a
// program is written with, not for strings a script computes.
func Intern(name string) Sym {
	if s, ok := LookupSym(name); ok {
		return s
	}
	syms.Lock()
	defer syms.Unlock()
	if s, ok := syms.byName[name]; ok {
		return s
	}
	s := Sym(len(syms.names))
	syms.names = append(syms.names, name)
	syms.byName[name] = s
	return s
}

// LookupSym returns the Sym of name, and false when nothing interned it:
// then no variable and no method of that name exists.
func LookupSym(name string) (Sym, bool) {
	syms.RLock()
	s, ok := syms.byName[name]
	syms.RUnlock()
	return s, ok
}

// String returns the name s stands for.
func (s Sym) String() string {
	syms.RLock()
	defer syms.RUnlock()
	return syms.names[s]
}
