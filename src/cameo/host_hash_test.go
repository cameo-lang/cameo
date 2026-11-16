package cameo

import (
	"reflect"
	"strings"
	"testing"
)

// A Hash whose keys are not all Strings or Symbols becomes a map[any]any
// for a host method. A binary String key is a []byte in Go, which a Go map
// cannot hold as a key: the call raises an ArgumentError the script can
// rescue, where it used to panic.
func TestHostMethodRejectsAnUnhashableKey(t *testing.T) {
	prog, out, calls := host(t, `begin
  Spicy.echo({ 1 => 2, b"k" => 1 })
rescue ArgumentError => e
  puts e.message
end
begin
  Spicy.echo({ 1 => 2, [1] => 1 })
rescue ArgumentError => e
  puts e.message
end
Spicy.echo({ 1 => 2, b"k" => 1 })`)
	err := prog.Run()
	if err == nil || !strings.Contains(err.Error(), "t.cameo:11: a Hash key of class String is a []uint8 in Go") {
		t.Errorf("err %v", err)
	}
	want := "a Hash key of class String is a []uint8 in Go, which a Go map cannot hold as a key\n" +
		"a Hash key of class Array is a []interface {} in Go, which a Go map cannot hold as a key\n"
	if out.String() != want {
		t.Errorf("output %q", out.String())
	}
	if len(*calls) != 0 {
		t.Errorf("the host method ran %d times", len(*calls))
	}
}

// f(1 => 2) passes its pairs as keywords, which a host method gets by
// name: a keyword that is not a String or a Symbol raises an
// ArgumentError, where it used to panic.
func TestHostMethodRejectsAKeywordThatIsNotAName(t *testing.T) {
	prog, out, calls := host(t, `begin
  Spicy.echo(1 => 2, b"k" => 1)
rescue ArgumentError => e
  puts e.message
end
Spicy.echo("a" => 1, b: 2)`)
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "a keyword of class Integer cannot reach a host method, whose keywords are names: pass a Hash in braces\n" {
		t.Errorf("output %q", out.String())
	}
	want := map[string]any{"a": int64(1), "b": int64(2)}
	if len(*calls) != 1 || !reflect.DeepEqual((*calls)[0][1], want) {
		t.Errorf("calls %#v", *calls)
	}
}

// Two keys that are distinct in cameo but one key in Go, as :a and "a"
// are once a Hash is a map[any]any, raise rather than lose one value.
func TestHostMethodRejectsKeysThatCollideInGo(t *testing.T) {
	prog, out, _ := host(t, `begin
  Spicy.echo({ 1 => 2, :a => 1, "a" => 2 })
rescue ArgumentError => e
  puts e.message
end`)
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "two keys of a Hash are both \"a\" in Go\n" {
		t.Errorf("output %q", out.String())
	}
}

// The same call from an async worker raises in that worker, where the
// worker's method rescues it, and in the script that awaits the worker
// when it does not; the process goes on.
func TestHostMethodRejectsAnUnhashableKeyInAWorker(t *testing.T) {
	prog, out, _ := host(t, `include Cameo
class Job
  def careful
    Spicy.echo({ 1 => 2, b"k" => 1 })
  rescue ArgumentError => e
    "rescued: #{e.message}"
  end

  def careless = Spicy.echo({ 1 => 2, b"k" => 1 })
end
job = Job.new
puts await(async(job, :careful))
begin
  await(async(job, :careless))
rescue ArgumentError => e
  puts "awaited: #{e.message}"
end`)
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	want := "rescued: a Hash key of class String is a []uint8 in Go, which a Go map cannot hold as a key\n" +
		"awaited: a Hash key of class String is a []uint8 in Go, which a Go map cannot hold as a key\n"
	if out.String() != want {
		t.Errorf("output %q", out.String())
	}
}

// Hashes a Go map can hold come through as before: String and Symbol keys
// as a map[string]any, a binary String among them, and other plain keys as
// a map[any]any.
func TestHostMethodGetsHashesWithPlainKeys(t *testing.T) {
	prog, _, calls := host(t, `Spicy.echo({ "a" => 1, :b => 2, b"c" => 3 })
Spicy.echo({ 1 => 2, 2.5 => :x, nil => true, :s => "t" })`)
	if err := prog.Run(); err != nil {
		t.Fatal(err)
	}
	want := []any{
		map[string]any{"a": int64(1), "b": int64(2), "c": int64(3)},
		map[any]any{int64(1): int64(2), 2.5: "x", nil: true, "s": "t"},
	}
	for i, w := range want {
		got := (*calls)[i][0].([]any)[0]
		if !reflect.DeepEqual(got, w) {
			t.Errorf("call %d: %#v", i, got)
		}
	}
}
