package cameo

import (
	"bytes"
	"errors"
	"testing"
)

func TestHostSettlesPromises(t *testing.T) {
	// A host method returns a promise and settles it later from another
	// goroutine: Resolve with a Go value, Fail with an error, which raises
	// its ExceptionClass or a RuntimeError.
	var out bytes.Buffer
	prog, err := Load("t.cameo", `include Cameo
p await(Later.value(41))
[Later.broken, Later.oops].each do |promise|
  await promise
rescue => e
  p [e.class, e.message]
end
`, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	later := prog.Module("Later")
	broken := later.Exception("Broken")
	settled := make(chan error, 3)
	later.Def("value", func(args []any, kwargs map[string]any) (any, error) {
		promise := prog.Promise()
		go func() {
			settled <- promise.Resolve(map[string]any{"n": args[0].(int64) + 1})
			if err := promise.Resolve(1); !errors.Is(err, ErrSettled) {
				t.Errorf("a second Resolve: %v", err)
			}
		}()
		return promise, nil
	})
	later.Def("broken", func(args []any, kwargs map[string]any) (any, error) {
		promise := prog.Promise()
		go func() { settled <- promise.Fail(broken.New("no %s", "luck")) }()
		return promise, nil
	})
	later.Def("oops", func(args []any, kwargs map[string]any) (any, error) {
		promise := prog.Promise()
		go func() { settled <- promise.Fail(errors.New("oops")) }()
		return promise, nil
	})
	if err := prog.Run(); err != nil {
		t.Fatalf("%v\noutput: %s", err, out.String())
	}
	for i := 0; i < 3; i++ {
		if err := <-settled; err != nil {
			t.Errorf("settling: %v", err)
		}
	}
	want := "{\"n\" => 42}\n[Later::Broken, \"no luck\"]\n[RuntimeError, \"oops\"]\n"
	if out.String() != want {
		t.Errorf("output %q, want %q", out.String(), want)
	}
}
