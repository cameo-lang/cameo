package object

import (
	"sync"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

func ints(l Value) []int64 {
	out := []int64{}
	for _, e := range l.List().Elems {
		out = append(out, e.Int())
	}
	return out
}

// Append leaves the list it appends to as it was, and what two appends
// to one list make are two lists: the second does not write where the
// first did.
func TestAppendLeavesEveryListAsItWas(t *testing.T) {
	a := NewList(Int(1), Int(2))
	b := Append(a.List(), Int(3))
	c := Append(b.List(), Int(4))
	d := Append(b.List(), Int(5), Int(6))
	e := Append(a.List())
	assert.Equals(t, ints(a), []int64{1, 2})
	assert.Equals(t, ints(b), []int64{1, 2, 3})
	assert.Equals(t, ints(c), []int64{1, 2, 3, 4})
	assert.Equals(t, ints(d), []int64{1, 2, 3, 5, 6})
	assert.Equals(t, ints(e), []int64{1, 2})
	// A Go append to a grown list's elements copies them.
	_ = append(c.List().Elems, Int(9))
	assert.Equals(t, ints(Append(c.List(), Int(7))), []int64{1, 2, 3, 4, 7})
}

// Appending to the list the last append made copies its elements only
// when the store is full: a thousand appends allocate far less than a
// thousand lists of five hundred elements.
func TestAppendInALoopDoesNotCopyTheList(t *testing.T) {
	var l Value
	allocs := testing.AllocsPerRun(10, func() {
		l = NewList()
		for i := range 1000 {
			l = Append(l.List(), Int(int64(i)))
		}
	})
	assert.Equals(t, l.List().Len(), 1000)
	assert.Equals(t, l.List().Elems[999].Int(), int64(999))
	// One header per append, and a store per growth.
	assert.True(t, allocs < 1040)
	b := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			l := NewList()
			for i := range 1000 {
				l = Append(l.List(), Int(int64(i)))
			}
		}
	})
	assert.True(t, b.AllocedBytesPerOp() < 200_000)
}

// Goroutines that append to one list each get a list of their own.
func TestAppendFromManyGoroutines(t *testing.T) {
	base := Append(NewList(Int(0)).List(), Int(1))
	var wg sync.WaitGroup
	got := make([]Value, 16)
	for g := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l := base
			for i := range 100 {
				l = Append(l.List(), Int(int64(g*1000+i)))
			}
			got[g] = l
		}()
	}
	wg.Wait()
	assert.Equals(t, ints(base), []int64{0, 1})
	for g, l := range got {
		es := ints(l)
		assert.Equals(t, len(es), 102)
		for i, e := range es[2:] {
			assert.Equals(t, e, int64(g*1000+i))
		}
	}
}
