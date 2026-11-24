package cameo

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Stateful is an object of the host whose state a dump carries: a deck of
// cards a host module deals from, a connection's cursor, a counter. The
// dump holds the script and nothing of the host, so a host keeps each such
// object with Keep, and the object turns its state into bytes for the dump
// and back on the resumed program.
type Stateful interface {
	// Dehydrate returns the object's state. Dump calls it once the run
	// paused and the calls async started settled, so no script code runs
	// alongside it.
	Dehydrate() ([]byte, error)
	// Rehydrate sets the object's state to what Dehydrate returned when the
	// dump was written. Keep calls it on a resumed program, before Run. It
	// may get the promises the dump holds with PendingPromise; it must not
	// otherwise call into the program.
	Rehydrate(data []byte) error
}

// kept is the host's objects a program keeps, and the state a resumed
// program got from its dump that no Keep has claimed yet.
type kept struct {
	mu        sync.Mutex
	objects   map[string]Stateful
	unclaimed map[string][]byte
}

// Keep has the program keep s under name: Dump stores the bytes s's
// Dehydrate returns under the name, and on a program Resume loaded from
// that dump, Keep of the same name hands them to s's Rehydrate before it
// returns. A name the dump holds no state for rehydrates nothing; on a
// program Load loaded, Keep only registers s.
//
// A resumed program refuses to Run while the dump holds state under a
// name no Keep has claimed, so a host that forgot an object hears of it
// before the script goes on without it. Keep an object before Run, and
// before you settle a promise the dump holds when settling it reads the
// object.
func (p *Program) Keep(name string, s Stateful) error {
	if name == "" {
		return errors.New("cameo: Keep needs a name")
	}
	if s == nil {
		return fmt.Errorf("cameo: Keep of %q needs an object", name)
	}
	p.kept.mu.Lock()
	defer p.kept.mu.Unlock()
	if _, ok := p.kept.objects[name]; ok {
		return fmt.Errorf("cameo: the program keeps an object under %q already", name)
	}
	if data, ok := p.kept.unclaimed[name]; ok {
		if err := s.Rehydrate(data); err != nil {
			return fmt.Errorf("cameo: rehydrating %q: %w", name, err)
		}
		delete(p.kept.unclaimed, name)
	}
	if p.kept.objects == nil {
		p.kept.objects = map[string]Stateful{}
	}
	p.kept.objects[name] = s
	return nil
}

// dehydrate returns the state of every object the program keeps, by name,
// and the state of a dump no Keep claimed: a program that resumes and
// dumps again without keeping an object passes its state on.
func (k *kept) dehydrate() (map[string][]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.objects) == 0 && len(k.unclaimed) == 0 {
		return nil, nil
	}
	out := make(map[string][]byte, len(k.objects)+len(k.unclaimed))
	for name, data := range k.unclaimed {
		out[name] = data
	}
	for _, name := range sortedNames(k.objects) {
		data, err := k.objects[name].Dehydrate()
		if err != nil {
			return nil, fmt.Errorf("cameo: dehydrating %q: %w", name, err)
		}
		if data == nil {
			data = []byte{}
		}
		out[name] = data
	}
	return out, nil
}

// restore takes the host state of a dump, for Keep to claim.
func (k *kept) restore(state map[string][]byte) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(state) == 0 {
		return
	}
	k.unclaimed = make(map[string][]byte, len(state))
	for name, data := range state {
		k.unclaimed[name] = data
	}
}

// checkClaimed fails, naming them, while the dump holds state no Keep
// claimed.
func (k *kept) checkClaimed() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if len(k.unclaimed) == 0 {
		return nil
	}
	names := sortedNames(k.unclaimed)
	return fmt.Errorf("cameo: the dump holds state for %s that no Keep claimed; keep each object before Run", strings.Join(quoted(names), ", "))
}

func sortedNames[V any](m map[string]V) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func quoted(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = fmt.Sprintf("%q", n)
	}
	return out
}
