package eval

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// A call async starts waits in the run's queue until a runner takes it, or
// until something awaits its promise and runs it itself; see Promise.wait.
// A goroutine per call would park, with its stack, every call whose
// awaiter runs it, and a recursion that fans out at every level starts
// millions.
//
// Runners are goroutines that take a call and run it: the oldest of one
// of the queue's shards, so calls near the root of a recursion tend to go
// first. Up to GOMAXPROCS of them run at once,
// started as calls arrive and ended when the queue is empty. Two rules
// keep every call able to run:
//
//   - A goroutine that is about to wait for a promise while calls are
//     queued starts another runner, so runners that wait on each other or
//     on a promise only a queued call settles still leave one free.
//   - A watchdog starts another runner whenever calls have waited for one
//     without any runner taking a call for a tick, so calls that never
//     end do not hold the queue up for good.

// tick is how long queued calls wait for a busy runner before the watchdog
// starts another.
const tick = 10 * time.Millisecond

type scheduler struct {
	shards [16]queueShard
	queued atomic.Int64
	// runners counts the runners; limit is how many push starts.
	runners atomic.Int32
	limit   int32
	// taken counts the calls runners took, which the watchdog reads to
	// see that they move.
	taken    atomic.Int64
	watching atomic.Bool
	next     atomic.Uint32
}

// queueShard is a FIFO of calls, linked through the promises.
type queueShard struct {
	mu         sync.Mutex
	head, tail *Promise
	// The padding keeps two shards' locks off one cache line.
	_ [40]byte
}

func newScheduler() *scheduler {
	return &scheduler{limit: int32(runtime.GOMAXPROCS(0))}
}

func (s *scheduler) shard(p *Promise) *queueShard {
	return &s.shards[uint64(p.id)%uint64(len(s.shards))]
}

// push queues the call of p and starts a runner when fewer than limit run.
func (s *scheduler) push(p *Promise) {
	q := s.shard(p)
	q.mu.Lock()
	p.linked = true
	s.queued.Add(1)
	if q.tail == nil {
		q.head = p
	} else {
		q.tail.next, p.prev = p, q.tail
	}
	q.tail = p
	q.mu.Unlock()
	for {
		n := s.runners.Load()
		if n >= s.limit {
			break
		}
		if s.runners.CompareAndSwap(n, n+1) {
			go s.run()
			break
		}
	}
	s.watch()
}

// claim takes p's call to run it, out of the queue; nil when a runner or
// an awaiter took it already.
func (s *scheduler) claim(p *Promise) *asyncCall {
	c := p.call.Swap(nil)
	if c == nil {
		return nil
	}
	q := s.shard(p)
	q.mu.Lock()
	if p.linked {
		s.unlink(q, p)
	}
	q.mu.Unlock()
	return c
}

// unlink takes p out of q, whose lock the caller holds.
func (s *scheduler) unlink(q *queueShard, p *Promise) {
	p.linked = false
	s.queued.Add(-1)
	if p.prev == nil {
		q.head = p.next
	} else {
		p.prev.next = p.next
	}
	if p.next == nil {
		q.tail = p.prev
	} else {
		p.next.prev = p.prev
	}
	p.prev, p.next = nil, nil
}

// pop takes the oldest call of a shard, starting at a different one each
// time; nil when the queue is empty.
func (s *scheduler) pop() (*Promise, *asyncCall) {
	start := s.next.Add(1)
	for i := range uint32(len(s.shards)) {
		q := &s.shards[(start+i)%uint32(len(s.shards))]
		q.mu.Lock()
		for p := q.head; p != nil; p = q.head {
			s.unlink(q, p)
			// An awaiter that swapped the call out first finds the promise
			// out of the queue already, and runs the call itself.
			if c := p.call.Swap(nil); c != nil {
				q.mu.Unlock()
				return p, c
			}
		}
		q.mu.Unlock()
	}
	return nil, nil
}

// run is a runner: it runs queued calls until there are none.
func (s *scheduler) run() {
	for {
		p, c := s.pop()
		if c == nil {
			s.runners.Add(-1)
			// A call pushed while this runner left found it still
			// counted, and started no other.
			if s.queued.Load() == 0 {
				return
			}
			s.runners.Add(1)
			continue
		}
		s.taken.Add(1)
		c.run(p)
	}
}

// waiting is called before a goroutine waits for a promise: with calls
// queued, it starts a runner in its place.
func (s *scheduler) waiting() {
	if s.queued.Load() > 0 {
		s.runners.Add(1)
		go s.run()
	}
}

// watch starts the watchdog unless it runs; it ends when the queue is
// empty.
func (s *scheduler) watch() {
	if s.watching.Load() || !s.watching.CompareAndSwap(false, true) {
		return
	}
	go func() {
		last := s.taken.Load()
		for {
			time.Sleep(tick)
			if s.queued.Load() == 0 {
				s.watching.Store(false)
				// A call pushed after the check found the watchdog still
				// running, and started no other.
				if s.queued.Load() == 0 || !s.watching.CompareAndSwap(false, true) {
					return
				}
			}
			if now := s.taken.Load(); now != last {
				last = now
				continue
			}
			s.runners.Add(1)
			go s.run()
		}
	}()
}
