package object

import "testing"

func TestKeyIsStructural(t *testing.T) {
	a := NewList(Int(1), String("x"), MapOf(Symbol("k"), Float(2.5)))
	b := NewList(Int(1), String("x"), MapOf(Symbol("k"), Float(2.5)))
	if Key(a) != Key(b) {
		t.Fatalf("equal lists have different keys: %s %s", Key(a), Key(b))
	}
	if Key(Int(1)) == Key(Float(1)) {
		t.Fatal("1 and 1.0 are different keys")
	}
	if Key(String("a")) == Key(Symbol("a")) {
		t.Fatal(`"a" and :a are different keys`)
	}
	if Key(NewSet(Int(1), Int(2))) != Key(NewSet(Int(2), Int(1))) {
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
	if m.Keys()[0] != String("b") || m.Values()[0] != Int(3) {
		t.Fatalf("a repeated key keeps its place and takes the last value: %v %v", m.Keys(), m.Values())
	}
	m2 := m.Put(String("c"), Int(4))
	if m.Len() != 2 || m2.Len() != 3 {
		t.Fatal("Put changed the original")
	}
	if m3 := m.Without(String("b")); m3.Len() != 1 || m.Len() != 2 {
		t.Fatal("Without changed the original")
	}
	if v, ok := m2.Get(String("c")); !ok || v != Int(4) {
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

func TestIntValueBoxesSmallIntegersOnce(t *testing.T) {
	for _, i := range []int64{-1023, -1, 0, 255, 256, 1024} {
		if IntValue(i) != Int(i) {
			t.Fatalf("IntValue(%d) = %v", i, IntValue(i))
		}
		if allocs := testing.AllocsPerRun(10, func() { _ = IntValue(i) }); allocs != 0 {
			t.Fatalf("IntValue(%d) allocates (%v per run)", i, allocs)
		}
	}
	for _, i := range []int64{-1024, 1025, 1 << 40} {
		if IntValue(i) != Int(i) {
			t.Fatalf("IntValue(%d) = %v", i, IntValue(i))
		}
	}
}
