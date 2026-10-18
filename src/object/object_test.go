package object

import (
	"math"
	"math/big"
	"testing"
)

func TestKeyIsStructural(t *testing.T) {
	a := NewList(Int(1), String("x"), MapValue(MapOf(Symbol("k"), Float(2.5))))
	b := NewList(Int(1), String("x"), MapValue(MapOf(Symbol("k"), Float(2.5))))
	if Key(a) != Key(b) {
		t.Fatalf("equal lists have different keys: %s %s", Key(a), Key(b))
	}
	if Key(Int(1)) == Key(Float(1)) {
		t.Fatal("1 and 1.0 are different keys")
	}
	if Key(String("a")) == Key(Symbol("a")) {
		t.Fatal(`"a" and :a are different keys`)
	}
	if Key(SetValue(NewSet(Int(1), Int(2)))) != Key(SetValue(NewSet(Int(2), Int(1)))) {
		t.Fatal("a set's key does not depend on insertion order")
	}
	if !Equal(Int(1), Float(1)) || Equal(Int(1), String("1")) {
		t.Fatal("Equal: numbers compare across kinds, nothing else does")
	}
}

func TestMapKeepsOrderAndIsImmutable(t *testing.T) {
	m := MapOf(String("b"), Int(1), String("a"), Int(2), String("b"), Int(3))
	if m.Len() != 2 {
		t.Fatalf("len %d", m.Len())
	}
	if !Equal(m.Keys()[0], String("b")) || !Equal(m.Values()[0], Int(3)) {
		t.Fatalf("a repeated key keeps its place and takes the last value: %v %v", m.Keys(), m.Values())
	}
	m2 := m.Put(String("c"), Int(4))
	if m.Len() != 2 || m2.Len() != 3 {
		t.Fatal("Put changed the original")
	}
	if m3 := m.Without(String("b")); m3.Len() != 1 || m.Len() != 2 {
		t.Fatal("Without changed the original")
	}
	if v, ok := m2.Get(String("c")); !ok || !Equal(v, Int(4)) {
		t.Fatal("Get after Put")
	}
}

func TestSetDropsDuplicates(t *testing.T) {
	s := NewSet(Int(1), Int(2), Int(1), NewList(Int(3)), NewList(Int(3)))
	if s.Len() != 3 {
		t.Fatalf("len %d", s.Len())
	}
	if !s.Has(NewList(Int(3))) {
		t.Fatal("a structurally equal list is a member")
	}
	u := s.Union(NewSet(Int(9)))
	if u.Len() != 4 || s.Len() != 3 {
		t.Fatal("Union changed the original")
	}
}

func TestFormatting(t *testing.T) {
	tenth, fifth := 0.1, 0.2
	cases := map[float64]string{1: "1.0", 2.5: "2.5", 1e20: "1.0e+20", 1.5e-7: "1.5e-07", tenth + fifth: "0.30000000000000004", -3: "-3.0"}
	for f, want := range cases {
		if got := FormatFloat(f); got != want {
			t.Errorf("FormatFloat(%v) = %s, want %s", f, got, want)
		}
	}
	if got := QuoteString("a\"b\\c\n\té"); got != `"a\"b\\c\n\té"` {
		t.Errorf("QuoteString: %s", got)
	}
	for s, want := range map[string]string{"name": ":name", "a?": ":a?", "+": ":+", "with space": `:"with space"`, "9x": `:"9x"`} {
		if got := SymbolInspect(s); got != want {
			t.Errorf("SymbolInspect(%q) = %s, want %s", s, got, want)
		}
	}
}

func TestAncestorsAndLookup(t *testing.T) {
	m := NewModule("M")
	m.Define("hello", 0, nil)
	a := NewClass("A", ObjectClass)
	b := NewClass("B", a)
	b.Include(m)
	anc := b.Ancestors()
	if anc[0] != b || anc[1] != m || anc[2] != a || anc[3] != ObjectClass {
		t.Fatalf("ancestors: %v", anc)
	}
	if b.Lookup("hello") == nil || a.Lookup("hello") != nil {
		t.Fatal("an included module's methods are found through the including class only")
	}
	if !b.IsA(m) || a.IsA(m) {
		t.Fatal("IsA sees included modules")
	}
}

func TestLookupCacheFollowsDefinitions(t *testing.T) {
	m := NewModule("Greets")
	a := NewClass("A", ObjectClass)
	b := NewClass("B", a)
	if b.Lookup("hello") != nil {
		t.Fatal("hello is not defined yet")
	}
	a.Define("hello", 0, nil)
	if b.Lookup("hello") == nil {
		t.Fatal("a method defined on the superclass after a miss is found")
	}
	if b.Lookup("bye") != nil {
		t.Fatal("bye is not defined yet")
	}
	m.Define("bye", 0, nil)
	a.Include(m)
	if b.Lookup("bye") == nil || !b.IsA(m) {
		t.Fatal("a module included after a miss is found, and shows in the ancestors")
	}
	if anc := b.Ancestors(); len(anc) != 4 || anc[1] != a || anc[2] != m {
		t.Fatalf("ancestors after include: %v", anc)
	}
}

func TestValuesAreTagged(t *testing.T) {
	if allocs := testing.AllocsPerRun(10, func() { _ = Int(1 << 40) }); allocs != 0 {
		t.Fatalf("Int allocates (%v per run)", allocs)
	}
	if allocs := testing.AllocsPerRun(10, func() { _ = Float(2.5) }); allocs != 0 {
		t.Fatalf("Float allocates (%v per run)", allocs)
	}
	if Int(7).Int() != 7 || Float(2.5).Float() != 2.5 || !True.Bool() || String("ab").Str() != "ab" || Symbol("k").Str() != "k" {
		t.Fatal("round trips")
	}
	if String("ab").Kind() != KindString || Symbol("ab").Kind() != KindSymbol || Equal(String("ab"), Symbol("ab")) {
		t.Fatal("a string and a symbol of the same text are different values")
	}
	var zero Value
	if !Same(Nil, Nil) || Same(Nil, zero) || !zero.IsZero() || zero.IsNil() || zero.Truthy() {
		t.Fatal("nil and the zero value are distinct, and both are falsy")
	}
	if s := "hello"; Same(String(s), String(s)) != true || Same(String(s), String(s[:5]+"")) != true {
		t.Fatal("strings sharing bytes are the same")
	}
	if Equal(Int(1), Float(1)) != true || Equal(Int(2), Float(1)) {
		t.Fatal("integers and floats compare by magnitude")
	}
	b := []byte("xyz")
	if string(Bytes(b).Bytes()) != "xyz" || Bytes(nil).Bytes() == nil || len(Bytes(nil).Bytes()) != 0 {
		t.Fatal("bytes round trip, and an empty byte string is an empty slice")
	}
}

func TestStringsCompareAcrossEncodings(t *testing.T) {
	// ASCII alone is the same string in every encoding; other bytes are
	// their encoding's, as in Ruby.
	if !Equal(String("ab"), Bytes([]byte("ab"))) || !Equal(String("ab"), StringOf(KindASCII, "ab")) || Key(String("ab")) != Key(Bytes([]byte("ab"))) {
		t.Fatal(`"ab" is "ab".b`)
	}
	if Equal(String("é"), Bytes([]byte("é"))) {
		t.Fatal(`"é" is not "é".b`)
	}
	if Bytes([]byte{0xff}).Class() != StringClass || StringOf(KindASCII, "x").Str() != "x" {
		t.Fatal("a binary or US-ASCII string is a String")
	}
}

func TestListKinds(t *testing.T) {
	// The kinds fit below the two bits Kinds keeps for itself.
	if KindProcess >= 30 {
		t.Fatalf("Kind %d does not fit in Kinds", KindProcess)
	}
	l := NewList(Int(1), Float(2), StringOf(KindASCII, "a"), Nil).List()
	if want := Kinds(1<<KindInt | 1<<KindFloat | 1<<KindASCII | 1<<KindNil); l.Kinds() != want || l.Kinds() != want {
		t.Fatalf("kinds %b, want %b", l.Kinds(), want)
	}
	if !NewList(Int(1), Int(2)).List().Kinds().Only(KindInt) {
		t.Fatal("a list of integers is not Only(KindInt)")
	}
	nan := NewList(Float(1), Float(math.NaN())).List().Kinds()
	if nan.Only(KindFloat) || nan&KindsNaN == 0 {
		t.Fatalf("a NaN is not marked: %b", nan)
	}
	if NewList().List().Kinds().Only(KindInt) {
		t.Fatal("an empty list is Only(KindInt)")
	}
	// A set keeps its kinds too, and a set made from it starts afresh.
	s := NewSet(Int(1), Int(2))
	if !s.Kinds().Only(KindInt) {
		t.Fatalf("set kinds %b", s.Kinds())
	}
	if w := s.With(Float(1.5)).Kinds(); w != 1<<KindInt|1<<KindFloat {
		t.Fatalf("kinds after With: %b", w)
	}
	if !s.With(Float(1.5)).Without(Float(1.5)).Kinds().Only(KindInt) || !s.Kinds().Only(KindInt) {
		t.Fatal("kinds after Without")
	}
}

func TestScalarRanges(t *testing.T) {
	big64 := BigInt(new(big.Int).Lsh(big.NewInt(1), 64)) // 2**64
	maxU64 := BigInt(new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(1)))
	cases := []struct {
		typ  string
		v    Value
		fits bool
	}{
		{"int32", Int(math.MaxInt32), true}, {"int32", Int(math.MaxInt32 + 1), false}, {"int32", Int(math.MinInt32), true}, {"int32", Int(math.MinInt32 - 1), false},
		{"sfixed32", Int(math.MinInt32 - 1), false}, {"sint64", Int(math.MinInt64), true},
		{"uint32", Int(math.MaxUint32), true}, {"uint32", Int(math.MaxUint32 + 1), false}, {"fixed32", Int(-1), false},
		{"uint64", maxU64, true}, {"fixed64", maxU64, true}, {"uint64", big64, false}, {"uint64", Int(-1), false},
		{"int", maxU64, false}, {"int64", Int(math.MaxInt64), true},
		{"bignum", big64, true}, {"bignum", Int(-5), true}, {"bignum", Float(1), false},
	}
	for _, c := range cases {
		if got := Scalar(c.typ).Fits(c.v); got != c.fits {
			t.Errorf("%s fits %s: %v, want %v", c.typ, IntegerDigits(c.v), got, c.fits)
		}
	}
	for _, name := range ScalarNames() {
		if ScalarField(name).Scalar != Scalar(name) {
			t.Errorf("ScalarField(%s) does not carry its scalar type", name)
		}
	}
	if Scalar("float").Proto != "double" || Scalar("float32").Proto != "float" || Scalar("int").Proto != "int64" || Scalar("bignum").Proto != "string" {
		t.Error("a cameo name maps to the wrong protobuf type")
	}
}
