package object

import (
	"strings"
	"unsafe"
)

// Enumerator is Ruby's Enumerator, and with Lazy set its Enumerator::Lazy:
// a sequence produced on demand. Each calls yield with each element in
// order and returns the first error yield returns, which is how a consumer
// stops it early; an endless source never returns otherwise. Like every
// value it never changes: a lazy method makes a new Enumerator whose Each
// runs this one's.
type Enumerator struct {
	Lazy bool
	Each func(ctx Context, yield func(Value) error) error
	// Recv, Meth and Args are what made it, for inspect: the receiver, or
	// the enumerator a lazy method was called on, and that method, if any.
	Recv Value
	Meth string
	Args []Value
	// Call, when set, calls that method again with a block, for each with
	// a block to return what the method returns, as Ruby's enum_for does.
	Call func(ctx Context, block Value) (Value, error)
	// Kw is the keywords of what made it, for inspect.
	Kw *Map
	// Size, when set, computes Enumerator#size without enumerating;
	// otherwise the size is nil.
	Size func(ctx Context) (Value, error)
	// Arith, when set, makes it an Enumerator::ArithmeticSequence, from
	// Numeric#step or Range#step: equal to another with the same Arith.
	Arith *ArithSeq
	// Of, when set, is its class, Enumerator::Chain or
	// Enumerator::Product, and Parts the enumerables it combines.
	Of    *Class
	Parts []Value
	// Make, when set, is the method that makes the enumerator again:
	// called on Recv with Args, Kw and Block it returns one with the same
	// elements. That call is what the enumerator is in a message, since
	// Each is Go code; an enumerator without it, and without Of, cannot
	// be serialised. For to_enum, Meth is its first argument.
	Make  string
	Block Value
}

// Made records the call that makes e again, and returns e: the method
// name called on e's Recv with its Args and Kw, and with block. A name
// that is not among the makers is not recorded, and the enumerator cannot
// be serialised.
func (e *Enumerator) Made(name string, block Value) *Enumerator {
	if MakesEnumerator(e.Recv, name, e.Args, block) {
		e.Make, e.Block = name, block
	}
	return e
}

// MakesEnumerator reports whether the call of name on recv with args and
// block is one that makes an enumerator and does nothing else: an
// iterating method called without a block, or a method of a lazy
// enumerator, which keeps its block for later. A reader makes an
// enumerator again by the call its message names, and makes no call but
// these, so a message cannot have it run a method that computes, or a
// block.
func MakesEnumerator(recv Value, name string, args []Value, block Value) bool {
	lazy := false
	if e, ok := recv.AsEnumerator(); ok {
		lazy = e.Lazy
	}
	if !block.IsZero() && !block.IsNil() && !lazy {
		return false
	}
	switch name {
	case "lazy":
		// lazy lists the elements of a script's object when it is made.
		return recv.Kind() != KindInstance
	case "%":
		return recv.Kind() == KindRange
	case "index", "find_index", "rindex":
		// With an argument these search.
		return len(args) == 0
	}
	return makers[name] || lazy && lazyMakers[name]
}

// makers are the methods of the core that return an enumerator when they
// are called without a block. A test (eval/enumerators_test.go) calls
// every method of the core's values without a block and fails for one
// that returns an enumerator and is not here.
var makers = names(`bsearch chunk classify collect collect_concat combination cycle
	detect divide downto drop_while each each_char each_cons each_entry
	each_index each_key each_line each_pair each_slice each_value
	each_with_index each_with_object filter filter_map find find_all
	flat_map group_by map max_by min_by minmax_by partition reject
	reverse_each select sort_by step take_while then times to_enum
	transform_keys transform_values upto with_index with_object yield_self`)

// lazyMakers are the methods of a lazy enumerator that return another:
// on any other receiver these compute.
var lazyMakers = names(`chunk_while compact drop eager grep grep_v slice_after
	slice_before slice_when take uniq zip`)

func names(list string) map[string]bool {
	out := map[string]bool{}
	for _, n := range strings.Fields(list) {
		out[n] = true
	}
	return out
}

// ArithSeq is what an Enumerator::ArithmeticSequence counts: from Begin
// by Step to End, which is nil for an endless one, and excluded when Excl
// is set. Begin is nil for one from a beginless range.
type ArithSeq struct {
	Begin, End, Step Value
	Excl             bool
}

// EnumeratorValue wraps an enumerator as a value.
func EnumeratorValue(e *Enumerator) Value { return Value{kind: KindEnumerator, p: unsafe.Pointer(e)} }

func (v Value) Enumerator() *Enumerator {
	v.must(KindEnumerator)
	return (*Enumerator)(v.p)
}

func (v Value) AsEnumerator() (*Enumerator, bool) {
	if v.kind != KindEnumerator {
		return nil, false
	}
	return (*Enumerator)(v.p), true
}
