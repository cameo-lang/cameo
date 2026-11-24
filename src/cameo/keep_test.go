package cameo

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/cameoio"
)

// deck is a host object a script deals from through a host module: its
// state is the cards left and the promise of the question it asked the
// player, which the dump does not hold.
type deck struct {
	Cards []string `json:"cards"`
	Asked int64    `json:"asked"`
}

func (d *deck) Dehydrate() ([]byte, error) { return json.Marshal(d) }

func (d *deck) Rehydrate(data []byte) error { return json.Unmarshal(data, d) }

// dealing loads the script with the Cards module over d: Cards.draw takes
// the top card, Cards.ask returns a promise the host settles, and
// Test.pause pauses the run.
func dealing(t *testing.T, prog *Program, d *deck) {
	t.Helper()
	cards := prog.Module("Cards")
	cards.Def("draw", func([]any, map[string]any, *Block) (any, error) {
		top := d.Cards[0]
		d.Cards = d.Cards[1:]
		return top, nil
	})
	cards.Def("ask", func([]any, map[string]any, *Block) (any, error) {
		pr := prog.Promise()
		d.Asked = pr.ID()
		return pr, nil
	})
	prog.Module("Test").Def("pause", func([]any, map[string]any, *Block) (any, error) {
		prog.Pause()
		return nil, nil
	})
}

const dealingScript = `include Cameo
puts Cards.draw
answer = Cards.ask
Test.pause
puts await(answer)
puts Cards.draw
`

// A host object the program keeps travels in the dump: the resumed
// program keeps a fresh one under the same name, which gets the state the
// first had when the dump was written, and the promise it held is the
// resumed program's pending promise of the same id.
func TestKeptStateResumes(t *testing.T) {
	var out bytes.Buffer
	prog, err := LoadFromString("t.cameo", dealingScript, cameoio.Streams(&out, &out))
	if err != nil {
		t.Fatal(err)
	}
	first := &deck{Cards: []string{"ace", "king", "queen"}}
	dealing(t, prog, first)
	if err := prog.Keep("deck", first); err != nil {
		t.Fatal(err)
	}
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	text, err := InspectDump(dump.Data)
	if err != nil {
		t.Fatal(err)
	}
	if want := "kept:\n  deck (36 bytes)\n"; !strings.HasSuffix(text, want) {
		t.Fatalf("inspect:\n%s\nwant it to end in:\n%s", text, want)
	}

	var rest bytes.Buffer
	cont, err := Resume(dump.Data, cameoio.Streams(&rest, &rest))
	if err != nil {
		t.Fatal(err)
	}
	second := &deck{}
	dealing(t, cont, second)
	if err := cont.Keep("deck", second); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(second.Cards, " "); got != "king queen" || second.Asked != first.Asked {
		t.Fatalf("the kept deck is %q asked %d, want \"king queen\" asked %d", got, second.Asked, first.Asked)
	}
	if err := cont.PendingPromise(second.Asked).Resolve("yes"); err != nil {
		t.Fatal(err)
	}
	if err := cont.Run(); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String()+rest.String(), "ace\nyes\nking\n"; got != want {
		t.Fatalf("the script printed %q, want %q", got, want)
	}
}

// A resumed program refuses to run while its dump holds state no Keep
// claimed, and runs once the host keeps the object.
func TestUnclaimedStateStopsRun(t *testing.T) {
	var out bytes.Buffer
	prog, err := LoadFromString("t.cameo", dealingScript, cameoio.Streams(&out, &out))
	if err != nil {
		t.Fatal(err)
	}
	d := &deck{Cards: []string{"ace", "king"}}
	dealing(t, prog, d)
	if err := prog.Keep("deck", d); err != nil {
		t.Fatal(err)
	}
	if err := prog.Keep("deck", d); err == nil || !strings.Contains(err.Error(), `keeps an object under "deck" already`) {
		t.Fatalf("a second Keep of deck returned %v", err)
	}
	if err := prog.Keep("", d); err == nil {
		t.Fatal("Keep with no name returned no error")
	}
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	cont, err := Resume(dump.Data, cameoio.Streams(&out, &out))
	if err != nil {
		t.Fatal(err)
	}
	again := &deck{}
	dealing(t, cont, again)
	err = cont.Run()
	if err == nil || err.Error() != `cameo: the dump holds state for "deck" that no Keep claimed; keep each object before Run` {
		t.Fatalf("Run returned %v", err)
	}
	if _, err := cont.Call(nil, "puts", "x"); err == nil {
		t.Fatal("Call ran a program whose state no Keep claimed")
	}
	// A name the dump holds nothing for rehydrates nothing.
	other := &deck{Cards: []string{"joker"}}
	if err := cont.Keep("other", other); err != nil || len(other.Cards) != 1 {
		t.Fatalf("Keep of other returned %v and left %v", err, other.Cards)
	}
	if err := cont.Keep("deck", again); err != nil {
		t.Fatal(err)
	}
	if err := cont.PendingPromise(again.Asked).Resolve("no"); err != nil {
		t.Fatal(err)
	}
	if err := cont.Run(); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "ace\nno\nking\n"; got != want {
		t.Fatalf("the script printed %q, want %q", got, want)
	}
}

// failing is a host object whose state does not travel.
type failing struct{}

func (failing) Dehydrate() ([]byte, error) { return nil, errors.New("no state") }
func (failing) Rehydrate([]byte) error     { return errors.New("bad state") }

// Dump fails when an object cannot give its state, naming it, and so does
// Keep when one cannot take it back.
func TestKeptStateErrors(t *testing.T) {
	prog, _ := pausing(t, "Test.pause\nputs 1\n")
	if err := prog.Keep("broken", failing{}); err != nil {
		t.Fatal(err)
	}
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	if _, err := prog.Dump(); err == nil || err.Error() != `cameo: dehydrating "broken": no state` {
		t.Fatalf("Dump returned %v", err)
	}

	prog, _ = pausing(t, "Test.pause\nputs 1\n")
	if err := prog.Keep("deck", &deck{}); err != nil {
		t.Fatal(err)
	}
	if err := prog.Run(); !errors.Is(err, ErrPaused) {
		t.Fatalf("Run returned %v, want ErrPaused", err)
	}
	dump, err := prog.Dump()
	if err != nil {
		t.Fatal(err)
	}
	cont, err := Resume(dump.Data, cameoio.Streams(&bytes.Buffer{}, &bytes.Buffer{}))
	if err != nil {
		t.Fatal(err)
	}
	if err := cont.Keep("deck", failing{}); err == nil || err.Error() != `cameo: rehydrating "deck": bad state` {
		t.Fatalf("Keep returned %v", err)
	}
}
