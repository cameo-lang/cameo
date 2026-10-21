package object

import "unsafe"

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
