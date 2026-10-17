package object

import (
	"strings"
	"sync"
	"testing"
)

// keysOf renders a map's keys and values in order, as "k=v,k=v".
func keysOf(m *Map) string {
	var b strings.Builder
	for i, k := range m.Keys() {
		b.WriteString(Key(k) + "=" + Key(m.Values()[i]) + ",")
	}
	return b.String()
}

func TestMapVersionsStayAsTheyWere(t *testing.T) {
	// Each Put grows the shared table at its tip; the older versions keep
	// seeing exactly their own entries.
	versions := []*Map{NewMap()}
	for i := 0; i < 100; i++ {
		versions = append(versions, versions[len(versions)-1].Put(Int(i), Int(i*i)))
	}
	for n, m := range versions {
		if m.Len() != n {
			t.Fatalf("version %d has %d entries", n, m.Len())
		}
		if _, ok := m.Get(Int(n)); ok {
			t.Fatalf("version %d sees a later key", n)
		}
		if n > 0 {
			if v, _ := m.Get(Int(n - 1)); !Equal(v, Int((n-1)*(n-1))) {
				t.Fatalf("version %d lost its last key", n)
			}
		}
	}
	// Writing through an old version branches: neither branch sees the
	// other's key.
	old := versions[10]
	a, b := old.Put(String("a"), Int(1)), old.Put(String("b"), Int(2))
	if a.Has(String("b")) || b.Has(String("a")) || old.Has(String("a")) || a.Len() != 11 || b.Len() != 11 {
		t.Fatal("branches from one version see each other's keys")
	}
}

func TestMapOverwriteDeleteAndOrder(t *testing.T) {
	m := MapOf(String("a"), Int(1), String("b"), Int(2), String("c"), Int(3))
	over := m.Put(String("a"), Int(9))
	if got := keysOf(over); got != `s1:a=i9,s1:b=i2,s1:c=i3,` {
		t.Fatalf("overwriting moves the key: %s", got)
	}
	if got := keysOf(m); got != `s1:a=i1,s1:b=i2,s1:c=i3,` {
		t.Fatalf("overwriting changed the original: %s", got)
	}
	gone := over.Without(String("a"))
	if gone.Has(String("a")) || gone.Len() != 2 || !over.Has(String("a")) {
		t.Fatal("Without changed the wrong version")
	}
	back := gone.Put(String("a"), Int(5))
	if got := keysOf(back); got != `s1:b=i2,s1:c=i3,s1:a=i5,` {
		t.Fatalf("a key added after its deletion goes last: %s", got)
	}
	if same := m.Put(String("a"), Int(1)); same.Len() != 3 || keysOf(same) != keysOf(m) {
		t.Fatal("setting a key to its value changes nothing")
	}
}

func TestMapCompactsOverwrites(t *testing.T) {
	// Overwriting one key many times keeps the table near the live size.
	m := NewMap()
	for i := 0; i < 10000; i++ {
		m = m.Put(String("k"), Int(i))
	}
	if n := len(m.t.entries); n > 64 {
		t.Fatalf("the table holds %d entries for one key", n)
	}
	if v, _ := m.Get(String("k")); !Equal(v, Int(9999)) || m.Len() != 1 {
		t.Fatal("compaction lost the latest value")
	}
}

func TestMapReadsWhileATipGrows(t *testing.T) {
	// A version read on other goroutines while the tip grows; run with
	// -race.
	base := NewMap()
	for i := 0; i < 1000; i++ {
		base = base.Put(Int(i), Int(i))
	}
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				if v, ok := base.Get(Int(i)); !ok || !Equal(v, Int(i)) {
					t.Errorf("lost key %d", i)
					return
				}
			}
			_ = base.Keys()
		}()
	}
	m := base
	for i := 1000; i < 5000; i++ {
		m = m.Put(Int(i), Int(i))
	}
	wg.Wait()
	if base.Len() != 1000 || m.Len() != 5000 {
		t.Fatal("lengths changed")
	}
}

func TestSetWithAndWithout(t *testing.T) {
	s := NewSet(Int(1), Int(2))
	s2 := s.With(Int(3)).With(Int(1)).Without(Int(2))
	if s.Len() != 2 || s2.Len() != 2 || !s2.Has(Int(3)) || s2.Has(Int(2)) || !s.Has(Int(2)) {
		t.Fatal("With and Without changed the wrong set")
	}
}

func TestMapShiftWalksForward(t *testing.T) {
	// Shifting the first key and re-adding it, as a queue does, keeps the
	// order and costs no scan of the keys already shifted.
	m := MapOf(Int(1), Int(10), Int(2), Int(20), Int(3), Int(30)).WithDefault(Int(0), Value{})
	for i := 0; i < 1000; i++ {
		k, v, rest, ok := m.Shift()
		if !ok {
			t.Fatal("shift of a full map failed")
		}
		m = rest.Put(k, v)
	}
	if got := keysOf(m); got != "i2=i20,i3=i30,i1=i10," {
		t.Fatalf("after 1000 rotations: %s", got)
	}
	if d, _ := m.Default(); !Equal(d, Int(0)) {
		t.Fatal("shift lost the default")
	}
	if _, _, _, ok := NewMap().Shift(); ok {
		t.Fatal("shift of an empty map")
	}
}
