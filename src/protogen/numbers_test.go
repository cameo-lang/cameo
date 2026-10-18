package protogen

import "testing"

func TestFieldNumber(t *testing.T) {
	none := func(int64) bool { return false }
	for _, name := range []string{"a", "x", "context", "created_at", "image_prompt"} {
		n := FieldNumber(name, none)
		if n < HashedNumberMin || n > HashedNumberMax || (n >= 19000 && n <= 19999) {
			t.Fatalf("%s: %d is out of range", name, n)
		}
		if again := FieldNumber(name, none); again != n {
			t.Fatalf("%s: %d, then %d", name, n, again)
		}
		if next := FieldNumber(name, func(m int64) bool { return m == n }); next != n+1 {
			t.Fatalf("%s: %d is taken, got %d, want the next one", name, n, next)
		}
	}
	if got := FieldNumber("x", none); got != 64843 {
		t.Fatalf("x is %d, want 64843", got)
	}
	if got := FieldNumber("x", func(m int64) bool { return m >= 64843 }); got != HashedNumberMin {
		t.Fatalf("past HashedNumberMax wraps to HashedNumberMin, got %d", got)
	}
}
