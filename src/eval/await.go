package eval

import (
	"sync"
	"sync/atomic"

	"github.com/cameo-lang/cameo/src/object"
)

// Waiting as state: an await does not hold its activation inside an
// instruction. The instruction says what the activation waits for, and
// the activation waits, so a pause finds it where it finds any other
// activation, and a dump holds what it waits for. doc/design/ir.md,
// "Workers and await at a pause", has the design.

// awaiting is what an activation waits for: promises, in order, and the
// results of the ones that settled.
type awaiting struct {
	ps []*Promise
	// single is set for an await of one promise, whose value is its
	// result; any other await's is the list of the results.
	single bool
	// next is the index of the promise the activation waits for now.
	next    int
	results []object.Value
	// failed is the first failure among the promises that settled, which
	// the await raises once every one of them has.
	failed *object.Exception
}

// await has the activation wait for w.
func (m *machine) await(w *awaiting) error {
	m.wait = w
	return m.awaited()
}

// awaited waits for what the activation awaits, and pushes the value of
// the await once it has it all. It returns ErrPaused, with the wait kept,
// when a pause wakes it where the run can pause.
func (m *machine) awaited() error {
	in, w := m.in, m.wait
	for w.next < len(w.ps) {
		p := w.ps[w.next]
		var v object.Value
		var exc *object.Exception
		var err error
		if in.pausable && in.loops == 1 && m.inGo == 0 {
			v, exc, err = p.waitOrPause(in)
		} else {
			v, exc, err = p.wait(in)
		}
		if err != nil {
			return err
		}
		w.next++
		if exc != nil {
			if w.failed == nil {
				w.failed = exc
			}
			continue
		}
		w.results = append(w.results, v)
	}
	m.wait = nil
	switch {
	case w.failed != nil:
		return w.failed
	case w.single:
		m.push(w.results[0])
	default:
		m.push(object.NewList(w.results...))
	}
	return nil
}

// pauser is a run's pause: a flag the loops read, and a channel that
// wakes what waits for a promise.
type pauser struct {
	flag atomic.Bool
	mu   sync.Mutex
	ch   chan struct{}
}

func newPauser() *pauser { return &pauser{ch: make(chan struct{})} }

// ask asks the run to pause.
func (p *pauser) ask() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.flag.Swap(true) {
		close(p.ch)
	}
}

// reset takes the pause back, for a run that goes on.
func (p *pauser) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.flag.Swap(false) {
		p.ch = make(chan struct{})
	}
}

// asked is closed once the run was asked to pause.
func (p *pauser) asked() <-chan struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ch
}

// waitOrPause is wait for a waiter that can pause: it returns ErrPaused
// when the run is asked to pause before the promise is settled.
func (p *Promise) waitOrPause(in *Interp) (object.Value, *object.Exception, error) {
	select {
	case <-p.done:
		return p.wait(in)
	default:
	}
	// A call that has not started runs here, as in wait.
	if c := in.promises.calls.claim(p); c != nil {
		c.run(p)
		if err := in.stopped(); err != nil {
			return object.Value{}, nil, err
		}
	}
	select {
	case <-p.done:
	default:
		in.promises.calls.waiting()
		select {
		case <-p.done:
		case <-in.stop.ch(p.id):
			return object.Value{}, nil, ErrStopped
		case <-in.pausing.asked():
			return object.Value{}, nil, ErrPaused
		}
	}
	return p.wait(in)
}
