package eval

import (
	"errors"
	"fmt"
	"maps"
	"math/big"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
	"github.com/cameo-lang/cameo/src/syntax"
)

// Promise is a result a script awaits: the call async started on a worker,
// or one a script, a worker or the host settles with resolve or fail. The
// script holds its id, an Integer, which travels to workers and to the host
// as any other value does.
type Promise struct {
	id int64
	// method is the method async runs; "" for a promise made by promise.
	method string
	done   chan struct{}
	// call is the call async started, until a runner or something that
	// awaits the promise takes it to run.
	call atomic.Pointer[asyncCall]
	// next and prev link the promise into the run's queue while linked;
	// the queue's shard lock guards them.
	next, prev *Promise
	linked     bool
	mu         sync.Mutex
	// settled is set by the first resolve or fail, or when the worker
	// returns; done closes then.
	settled bool
	result  object.Value
	err     *object.Exception
	// later, when set, makes the result on the goroutine that awaits: the
	// host settles with Go values, which only an interpreter converts.
	later func(*Interp) (object.Value, error)
	// awaited is set once something waited for the promise, so await :all
	// and the end of the run know whose failure nobody saw.
	awaited bool
	// ended is set when the method that started the promise returned
	// before anything awaited it: the first await then forgets it.
	ended bool
}

// ID is the promise's id, the Integer a script awaits.
func (p *Promise) ID() int64 { return p.id }

// settle settles the promise once; it reports false when it was settled
// already.
func (p *Promise) settle(result object.Value, exc *object.Exception, later func(*Interp) (object.Value, error)) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.settled {
		return false
	}
	p.settled, p.result, p.err, p.later = true, result, exc, later
	close(p.done)
	return true
}

// SettleLater settles the promise with a result made on the goroutine that
// awaits it: an error that is an *object.Exception raises it, any other a
// RuntimeError. It reports false when the promise was settled already.
func (p *Promise) SettleLater(f func(*Interp) (object.Value, error)) bool {
	return p.settle(object.Value{}, nil, f)
}

// wait blocks until the promise is settled, or the run is stopped, and
// returns its result or the exception it failed with.
func (p *Promise) wait(in *Interp) (object.Value, *object.Exception, error) {
	// A call that has not started runs here rather than on its goroutine,
	// so the goroutine that awaits it does not park, with its stack, while
	// another runs it.
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
		// A settled promise does not take the stop channel's lock.
		select {
		case <-p.done:
		case <-in.stop.ch(p.id):
			return object.Value{}, nil, ErrStopped
		}
	}
	p.mu.Lock()
	if p.later != nil {
		v, err := p.later(in)
		p.later = nil
		p.result, p.err = v, asException(in, err)
	}
	p.awaited = true
	result, exc, forget := p.result, p.err, p.ended
	p.mu.Unlock()
	if forget {
		in.promises.remove(p.id)
	}
	return result, exc, nil
}

func asException(in *Interp, err error) *object.Exception {
	if err == nil {
		return nil
	}
	if exc, ok := err.(*object.Exception); ok {
		return exc
	}
	return in.raise(object.RuntimeError, 0, "%s", err.Error()).(*object.Exception)
}

// promises is every promise of a run, shared with its workers, so a worker
// settles a promise its parent awaits. Ids count up from 1 in start order.
// The table is split into shards by id, so goroutines that start and await
// promises at the same time rarely take the same lock.
type promises struct {
	next atomic.Int64
	// The padding keeps the counter off shard 0's cache line.
	_      [56]byte
	shards [shards]promiseShard
	// calls is the queue of the calls async started; see sched.go.
	calls *scheduler
}

// shards is how many parts the promise table and the run's stop channels
// are split into; shardOf picks the part of promise id in both.
const shards = 64

func shardOf(id int64) int { return int(uint64(id) % shards) }

type promiseShard struct {
	mu   sync.Mutex
	byID map[int64]*Promise
	// The padding keeps two shards' locks off one cache line.
	_ [48]byte
}

func newPromises() *promises {
	t := &promises{calls: newScheduler()}
	for i := range t.shards {
		t.shards[i].byID = map[int64]*Promise{}
	}
	return t
}

func (t *promises) shard(id int64) *promiseShard {
	return &t.shards[shardOf(id)]
}

func (t *promises) add(method string) *Promise {
	p := &Promise{id: t.next.Add(1), method: method, done: make(chan struct{})}
	s := t.shard(p.id)
	s.mu.Lock()
	s.byID[p.id] = p
	s.mu.Unlock()
	return p
}

// restore puts a promise a dump held back, keeping the ids counting on
// past it.
func (t *promises) restore(p *Promise) {
	s := t.shard(p.id)
	s.mu.Lock()
	s.byID[p.id] = p
	s.mu.Unlock()
	for {
		n := t.next.Load()
		if n >= p.id || t.next.CompareAndSwap(n, p.id) {
			return
		}
	}
}

func (t *promises) remove(id int64) {
	s := t.shard(id)
	s.mu.Lock()
	delete(s.byID, id)
	s.mu.Unlock()
}

// end forgets the promises a method started, as it returns: those it
// awaited now, the others when they are awaited. Their ids may have left
// the method, but a script has no use for a promise once it was awaited.
func (t *promises) end(ps []*Promise) {
	for _, p := range ps {
		p.mu.Lock()
		awaited := p.awaited
		p.ended = !awaited
		p.mu.Unlock()
		if awaited {
			t.remove(p.id)
		}
	}
}

func (t *promises) get(id int64) *Promise {
	s := t.shard(id)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byID[id]
}

// NewPromise makes a pending promise, which the host settles from any
// goroutine and hands the script by its ID.
func (in *Interp) NewPromise() *Promise { return in.promises.add("") }

// waitStarted waits for every promise async started, in start order, and
// writes the failures nobody awaited to standard error.
func (in *Interp) waitStarted() error {
	t := in.promises
	for id := int64(1); id <= t.next.Load(); id++ {
		// A promise that is gone was awaited. One that add counted and has
		// not stored yet belongs to a worker whose own promise, with a
		// lower id, was waited for already, so it cannot be pending.
		p := t.get(id)
		if p == nil || p.method == "" {
			continue
		}
		p.mu.Lock()
		seen := p.awaited
		p.mu.Unlock()
		_, exc, err := p.wait(in)
		if err != nil {
			return err
		}
		if exc != nil && !seen {
			in.Write(true, fmt.Sprintf("cameo: promise %d (%s) failed and was never awaited: %s (%s)\n", p.id, p.method, exc.Message, exc.Cls.QualifiedName()))
		}
	}
	return nil
}

// cameoIncluded reports whether async and the rest of Cameo are in scope:
// the script included Cameo at the top level, or self's class did.
func (in *Interp) cameoIncluded(fr *Frame) bool {
	return in.cameoTop || fr.Self.Class().IsA(object.CameoModule)
}

func (in *Interp) needCameo(qualified bool, name string, fr *Frame, line int) error {
	if qualified || in.cameoIncluded(fr) {
		return nil
	}
	return in.raise(object.NoMethodError, line, "%s comes from the Cameo module: include Cameo, or call Cameo.%s", name, name)
}

// evalAsync starts recv.method(*args) on a worker and returns its promise.
func (in *Interp) evalAsync(n *syntax.Async, env *Env, fr *Frame) (object.Value, error) {
	if err := in.needCameo(n.Qualified, "async", fr, n.Line()); err != nil {
		return object.Value{}, err
	}
	p, err := in.start(n.Recv, n.Method, n.Args, n.KwArgs, env, fr, n.Line())
	if err != nil {
		return object.Value{}, err
	}
	return object.Int(p.id), nil
}

// evalAwait waits for a promise or a list of them, or starts a call and
// waits for it.
func (in *Interp) evalAwait(n *syntax.Await, env *Env, fr *Frame) (object.Value, error) {
	if err := in.needCameo(n.Qualified, "await", fr, n.Line()); err != nil {
		return object.Value{}, err
	}
	if n.Promise == nil {
		p, err := in.start(n.Recv, n.Method, n.Args, n.KwArgs, env, fr, n.Line())
		if err != nil {
			return object.Value{}, err
		}
		return in.awaitOne(p)
	}
	v, err := in.eval(n.Promise, env, fr)
	if err != nil {
		return object.Value{}, err
	}
	if l, ok := v.AsList(); ok {
		ps := make([]*Promise, l.Len())
		for i := range ps {
			if ps[i], err = in.promise(l.Elems[i], n.Line()); err != nil {
				return object.Value{}, err
			}
		}
		return in.awaitEach(ps)
	}
	p, err := in.promise(v, n.Line())
	if err != nil {
		return object.Value{}, err
	}
	return in.awaitOne(p)
}

// promise is the promise an id names.
func (in *Interp) promise(v object.Value, line int) (*Promise, error) {
	id, ok := v.AsInt()
	if !ok {
		return nil, in.raise(object.TypeError, line, "a promise is the Integer async or promise returned, not %s", typeName(v))
	}
	p := in.promises.get(id)
	if p == nil {
		return nil, in.raise(object.ArgumentError, line, "no promise %d", id)
	}
	return p, nil
}

func (in *Interp) awaitOne(p *Promise) (object.Value, error) {
	result, exc, err := p.wait(in)
	if err != nil {
		return object.Value{}, err
	}
	if exc != nil {
		return object.Value{}, exc
	}
	return result, nil
}

// awaitEach waits for every promise and returns their results in order;
// when one failed it raises the first failure after all are settled.
func (in *Interp) awaitEach(ps []*Promise) (object.Value, error) {
	results := make([]object.Value, 0, len(ps))
	var first *object.Exception
	for _, p := range ps {
		v, exc, err := p.wait(in)
		if err != nil {
			return object.Value{}, err
		}
		if exc != nil {
			if first == nil {
				first = exc
			}
			continue
		}
		results = append(results, v)
	}
	if first != nil {
		return object.Value{}, first
	}
	return object.NewList(results...), nil
}

// evalAwaitAll waits for every promise async started in this scope that is
// not awaited yet, and returns their results in start order.
func (in *Interp) evalAwaitAll(n *syntax.AwaitAll, env *Env, fr *Frame) (object.Value, error) {
	if err := in.needCameo(n.Qualified, "await", fr, n.Line()); err != nil {
		return object.Value{}, err
	}
	var pending []*Promise
	for _, p := range methodScope(env).promises {
		p.mu.Lock()
		if !p.awaited {
			pending = append(pending, p)
		}
		p.mu.Unlock()
	}
	return in.awaitEach(pending)
}

// methodScope is the scope a promise belongs to: the method body's, or the
// top level's, that the current block scope sits in. A block is part of
// the method that runs it, so a method that fans out in a loop and then
// waits sees every promise.
func methodScope(env *Env) *Env {
	for env.parent != nil {
		env = env.parent
	}
	return env
}

// start serialises the receiver, the method name and the arguments, and
// queues the call for a worker interpreter; see sched.go.
func (in *Interp) start(recvNode, methodNode syntax.Node, argNodes []syntax.Node, kwNodes []syntax.MapEntry, env *Env, fr *Frame, line int) (*Promise, error) {
	recv, err := in.eval(recvNode, env, fr)
	if err != nil {
		return nil, err
	}
	m, err := in.eval(methodNode, env, fr)
	if err != nil {
		return nil, err
	}
	var method string
	switch m.Kind() {
	case object.KindSymbol, object.KindString, object.KindBytes, object.KindASCII:
		method = m.Str()
	default:
		return nil, in.raise(object.TypeError, line, "async names the method by a symbol, not %s", typeName(m))
	}
	args, kw, err := in.evalCallArgs(argNodes, kwNodes, env, fr, line)
	if err != nil {
		return nil, err
	}
	if !in.RespondsTo(recv, method) {
		return nil, in.raise(object.NoMethodError, line, "undefined method '%s' for %s", method, typeName(recv))
	}
	enc := in.Encoder()
	encode := func(v object.Value) ([]byte, error) {
		data, err := enc.Encode(v)
		var ns *protogen.NotSerialisable
		if errors.As(err, &ns) {
			return nil, in.raise(object.TypeError, line, "a %s cannot be passed to async", ns.Kind)
		}
		if err != nil {
			return nil, in.raise(object.TypeError, line, "%s", err.Error())
		}
		return data, nil
	}
	cross := func(v object.Value) (crossing, error) {
		if in.readsBackAsIs(v) {
			if v.Kind() == object.KindInstance {
				// ToMessage keeps an instance's message in the instance,
				// so the worker gets an instance of its own to write to.
				v = object.InstanceValue(&object.Instance{Cls: v.Instance().Cls})
			}
			return crossing{v: v}, nil
		}
		data, err := encode(v)
		return crossing{data: data, wire: true}, err
	}
	recvC, err := cross(recv)
	if err != nil {
		return nil, err
	}
	argC := make([]crossing, len(args))
	for i, a := range args {
		if argC[i], err = cross(a); err != nil {
			return nil, err
		}
	}
	var kwData []byte
	if kw != nil {
		if kwData, err = encode(object.MapValue(kw)); err != nil {
			return nil, err
		}
	}
	p := in.promises.add(method)
	scope := methodScope(env)
	scope.promises = append(scope.promises, p)
	p.call.Store(&asyncCall{w: in.worker(), recv: recvC, method: method, args: argC, kw: kwData})
	in.promises.calls.push(p)
	return p, nil
}

// asyncCall is a call async started, with the worker interpreter it runs
// in.
type asyncCall struct {
	w      *Interp
	recv   crossing
	method string
	args   []crossing
	kw     []byte
}

// run makes the call and settles p with its result.
func (c *asyncCall) run(p *Promise) {
	result, exc := c.w.runCall(c.recv, c.method, c.args, c.kw)
	if exc != nil {
		p.settle(object.Value{}, exc, nil)
		return
	}
	p.settle(result, nil, nil)
}

// topLevel is a copy of an interpreter's top level: its constants, its
// top-level methods and whether it included Cameo. Workers share one and
// only read it.
type topLevel struct {
	consts    map[string]object.Value
	mainClass *object.Class
	main      *object.Instance
	cameoTop  bool
}

// snapshot returns the top level for a worker: the one taken for the
// last worker, or a new copy when the top level changed since.
func (in *Interp) snapshot() *topLevel {
	if in.snap == nil {
		mainClass := copyMainClass(in.mainClass)
		in.snap = &topLevel{
			consts:    maps.Clone(in.consts),
			mainClass: mainClass,
			main:      &object.Instance{Cls: mainClass},
			cameoTop:  in.cameoTop,
		}
	}
	return in.snap
}

// changedTop drops the snapshot after the top level changed, so the next
// worker takes a new one. Only the parent changes its top level: a worker
// runs a method, and def, class, module and constant assignment are
// rejected inside a method.
func (in *Interp) changedTop() { in.snap = nil }

// copyMainClass returns a new class of the top-level self with c's methods
// and includes.
func copyMainClass(c *object.Class) *object.Class {
	mainClass := object.NewClass("Object", object.ObjectClass)
	mainClass.Digest = "main"
	// The new class has no lookups cached yet, so its methods go in
	// directly: SetMethod would invalidate every class's cache.
	maps.Copy(mainClass.Methods, c.Methods)
	mainClass.Includes = append(mainClass.Includes, c.Includes...)
	return mainClass
}

// worker makes the interpreter a call async started runs in: the same
// output, the same classes (immutable once bound), the top level as it is
// now, the same promises, its own scopes.
func (in *Interp) worker() *Interp {
	top := in.snapshot()
	w := &Interp{
		IgnoreMutationErrors: in.IgnoreMutationErrors,
		stdout:               in.stdout,
		stderr:               in.stderr,
		outMu:                in.outMu,
		file:                 in.file,
		mainClass:            top.mainClass,
		main:                 top.main,
		top:                  NewEnv(nil),
		consts:               top.consts,
		proto:                in.proto,
		promises:             in.promises,
		cameoTop:             top.cameoTop,
		code:                 in.code,
		stop:                 in.stop,
		snap:                 top,
	}
	// The worker's generator is seeded from this one, here on the
	// parent's goroutine, so a seeded script stays reproducible.
	seed := uint64(in.rng.Uint32())<<32 | uint64(in.rng.Uint32())
	w.rng = object.NewRandom(object.BigInt(new(big.Int).SetUint64(seed)))
	return w
}

// runCall decodes the receiver and the arguments, and calls the method.
func (w *Interp) runCall(recvC crossing, method string, argC []crossing, kwData []byte) (object.Value, *object.Exception) {
	dec := w.Decoder()
	recv, err := recvC.value(dec)
	if err != nil {
		return object.Value{}, w.raise(object.TypeError, 0, "%s", err.Error()).(*object.Exception)
	}
	args := make([]object.Value, len(argC))
	for i, c := range argC {
		if args[i], err = c.value(dec); err != nil {
			return object.Value{}, w.raise(object.TypeError, 0, "%s", err.Error()).(*object.Exception)
		}
	}
	var kw *object.Map
	if kwData != nil {
		v, err := dec.Decode(kwData)
		if err != nil {
			return object.Value{}, w.raise(object.TypeError, 0, "%s", err.Error()).(*object.Exception)
		}
		// Keywords are symbols, which travelled as strings.
		if m, ok := v.AsMap(); ok {
			b := object.NewMapBuilder()
			for i, k := range m.Keys() {
				if s, isStr := k.AsString(); isStr {
					k = object.Symbol(s)
				}
				b.Set(k, m.Values()[i])
			}
			kw = b.Map()
		}
	}
	fr := &Frame{Self: object.InstanceValue(w.main), DefTarget: w.mainClass}
	result, callErr := w.call(fr, recv, method, args, kw, object.Value{}, false, 0)
	if callErr != nil {
		if exc, ok := callErr.(*object.Exception); ok {
			return object.Value{}, exc
		}
		if c, ok := callErr.(*control); ok {
			return object.Value{}, w.raise(object.LocalJumpError, c.line, "%s from a worker", c.kind).(*object.Exception)
		}
		return object.Value{}, w.raise(object.CameoError, 0, "%s", callErr.Error()).(*object.Exception)
	}
	if exc := w.travels(result, "a worker returned"); exc != nil {
		return object.Value{}, exc
	}
	return result, nil
}

// crossing is a value on its way to a worker: the value itself when it
// reads back from the envelope unchanged, its wire form otherwise.
type crossing struct {
	v    object.Value
	data []byte
	wire bool
}

// value returns the value the worker gets.
func (c crossing) value(dec *protogen.Decoder) (object.Value, error) {
	if !c.wire {
		return c.v, nil
	}
	return dec.Decode(c.data)
}

// readsBackAsIs reports whether v is equal to what a worker decodes from
// its wire form, so the worker can take v itself: values are immutable,
// and the round trip costs more than most calls async starts. A symbol
// reads back as a string and a hash's symbol keys as strings, so neither
// is one, nor is a string that is not valid UTF-8, which the envelope
// rejects. An instance is one when it has no fields and its class is one
// the script defined and the worker finds by name and digest; a list is
// one when it holds no instance.
func (in *Interp) readsBackAsIs(v object.Value) bool {
	switch v.Kind() {
	case object.KindBool, object.KindInt, object.KindBig, object.KindFloat:
		return true
	case object.KindString:
		return utf8.ValidString(v.Str())
	case object.KindList:
		for _, e := range v.List().Elems {
			if e.Kind() == object.KindInstance || !in.readsBackAsIs(e) {
				return false
			}
		}
		return true
	case object.KindInstance:
		inst := v.Instance()
		cls := inst.Cls
		return len(cls.Fields) == 0 && scriptDefined(cls) && in.FindClass(cls.QualifiedName(), cls.Digest) == cls
	}
	return v.IsNil()
}

// scriptDefined reports whether the script defined cls: a builtin class
// has no digest, and one the host registered a "host:" one.
func scriptDefined(cls *object.Class) bool {
	return cls.Digest != "" && !strings.HasPrefix(cls.Digest, "host:")
}

// travels is nil when v can cross to another interpreter, the exception to
// raise when it cannot: a Proc or a class stays where it was made.
func (in *Interp) travels(v object.Value, what string) *object.Exception {
	if in.readsBackAsIs(v) {
		return nil
	}
	_, err := in.Encoder().Encode(v)
	if err == nil {
		return nil
	}
	var ns *protogen.NotSerialisable
	if errors.As(err, &ns) {
		return in.raise(object.TypeError, 0, "%s a %s, which cannot be passed back", what, ns.Kind).(*object.Exception)
	}
	return in.raise(object.TypeError, 0, "%s", err.Error()).(*object.Exception)
}

// installCameo defines the methods of Cameo besides async and await:
// promise, resolve and fail, as module functions and, for a script that
// includes Cameo, instance methods. fail with an Integer first settles a
// promise; anything else raises, as Kernel#fail does.
func installCameo() {
	c := object.CameoModule
	define := func(name string, arity int, fn object.NativeFn) {
		c.Define(name, arity, fn)
		c.DefineSingleton(name, arity, fn)
	}
	define("promise", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(ctx.(*Interp).NewPromise().id), nil
	})
	define("resolve", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		in := ctx.(*Interp)
		p, err := in.promise(args[0], 0)
		if err != nil {
			return object.Value{}, err
		}
		if exc := in.travels(args[1], "resolve got"); exc != nil {
			return object.Value{}, exc
		}
		if !p.settle(args[1], nil, nil) {
			return object.Value{}, in.raise(object.ArgumentError, 0, "promise %d is settled already", p.id)
		}
		return args[0], nil
	})
	raise := object.ObjectClass.Methods["fail"].Native
	fail := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		in := ctx.(*Interp)
		if len(args) < 2 || args[0].Kind() != object.KindInt {
			return raise(ctx, self, args, kw, block)
		}
		p, err := in.promise(args[0], 0)
		if err != nil {
			return object.Value{}, err
		}
		// The failure is the exception raise would raise with the rest of
		// the arguments.
		_, raised := raise(ctx, self, args[1:], kw, block)
		exc, ok := raised.(*object.Exception)
		if !ok {
			return object.Value{}, raised
		}
		if !p.settle(object.Value{}, exc, nil) {
			return object.Value{}, in.raise(object.ArgumentError, 0, "promise %d is settled already", p.id)
		}
		return args[0], nil
	}
	define("fail", -1, fail)
	// A script that includes Cameo at the top level reaches fail from any
	// method, where Kernel's would answer first.
	object.ObjectClass.Define("fail", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if in, ok := ctx.(*Interp); ok && (in.cameoTop || self.Class().IsA(object.CameoModule)) {
			return fail(ctx, self, args, kw, block)
		}
		return raise(ctx, self, args, kw, block)
	})
}
