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
		if in.canPause(m) {
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
// wakes what waits for a promise. It holds the workers that froze for the
// pause, by the promise each settles, and tells Dump when one settled,
// froze or was thawed.
type pauser struct {
	flag   atomic.Bool
	mu     sync.Mutex
	ch     chan struct{}
	cond   *sync.Cond
	frozen map[*Promise]*Interp
	// stop is the run's stop, whose interrupt flag a pause sets too, for
	// compiled code; nil for a pauser no run has.
	stop *stopper
}

func newPauser() *pauser {
	p := &pauser{ch: make(chan struct{}), frozen: map[*Promise]*Interp{}}
	p.cond = sync.NewCond(&p.mu)
	return p
}

// changed tells what waits for the run to be quiet that a worker settled
// its promise.
func (p *pauser) changed() {
	p.mu.Lock()
	p.cond.Broadcast()
	p.mu.Unlock()
}

// need marks the promise pr as needed: something that cannot pause waits
// for it. Its worker does not freeze, and one that froze goes on.
func (p *pauser) need(pr *Promise) {
	p.mu.Lock()
	pr.needed.Store(true)
	w := p.frozen[pr]
	delete(p.frozen, pr)
	p.mu.Unlock()
	if w != nil {
		go w.thaw(pr)
	}
}

// thawAll has every worker that froze go on: the run goes on.
func (p *pauser) thawAll() {
	p.mu.Lock()
	frozen := p.frozen
	p.frozen = map[*Promise]*Interp{}
	p.mu.Unlock()
	for pr, w := range frozen {
		go w.thaw(pr)
	}
}

// goOnPaused takes the pause back for a run that goes on: the workers
// that froze go on, the queue moves again, and no promise is needed any
// more.
func (in *Interp) goOnPaused() {
	in.pausing.reset()
	t := in.promises
	for id := int64(1); id <= t.next.Load(); id++ {
		if p := t.get(id); p != nil {
			p.needed.Store(false)
		}
	}
	in.pausing.thawAll()
	t.calls.kick()
}

// quiet waits until no call async started is running: each is settled,
// still queued, or its worker froze and nothing needs it. Nothing can
// come to need a frozen worker then, and Dump reads the workers.
func (in *Interp) quiet() {
	ps, t := in.pausing, in.promises
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for {
		running := false
		for id := int64(1); id <= t.next.Load() && !running; id++ {
			p := t.get(id)
			if p == nil || p.method == "" || p.call.Load() != nil {
				continue
			}
			p.mu.Lock()
			settled := p.settled
			p.mu.Unlock()
			if !settled && (ps.frozen[p] == nil || p.needed.Load()) {
				running = true
			}
		}
		if !running {
			return
		}
		ps.cond.Wait()
	}
}

// freeze keeps the activations of the interpreter as they are, for the
// dump, and reports true: the interpreter stops. A worker whose promise
// something needs does not freeze.
func (in *Interp) freeze() bool {
	acts := append([]*machine(nil), in.acts...)
	if !in.isWorker {
		in.frozen = acts
		return true
	}
	if in.settles.needed.Load() {
		return false
	}
	// The worker is frozen for the others once its goroutine has left
	// it: finish says so, as the last thing that goroutine does.
	in.frozen = acts
	return true
}

// frozenNow says the worker w, which froze, has left its goroutine: Dump
// may read it, and a need of its promise thaws it. It reports false when
// the promise came to be needed meanwhile: the worker goes on at once.
func (p *pauser) frozenNow(w *Interp, pr *Promise) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if pr.needed.Load() {
		return false
	}
	p.frozen[pr] = w
	p.cond.Broadcast()
	return true
}

// canPause reports whether the interpreter stops for a pause where it
// is, before an instruction of m: one loop runs, m is in no list of its
// own, and nothing needs the promise of a worker.
func (in *Interp) canPause(m *machine) bool {
	return in.pausable && in.loops == 1 && m.inGo == 0 && !(in.isWorker && in.settles.needed.Load())
}

// ask asks the run to pause.
func (p *pauser) ask() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.flag.Swap(true) {
		close(p.ch)
	}
	if p.stop != nil {
		p.stop.interrupt.Store(true)
	}
}

// reset takes the pause back, for a run that goes on.
func (p *pauser) reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.flag.Swap(false) {
		p.ch = make(chan struct{})
	}
	if p.stop != nil {
		// A run that was stopped meanwhile stays interrupted.
		p.stop.interrupt.Store(p.stop.flag.Load())
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
	// A call that has not started runs here, as in wait, unless a pause
	// is asked for: then it stays queued, for the dump.
	if !in.pausing.flag.Load() {
		if c := in.promises.calls.claim(p); c != nil {
			c.run(p)
			if err := in.stopped(); err != nil {
				return object.Value{}, nil, err
			}
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
