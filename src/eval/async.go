package eval

import (
	"errors"
	"fmt"
	"sync"

	"github.com/radiospiel/amber/src/object"
	"github.com/radiospiel/amber/src/protogen"
	"github.com/radiospiel/amber/src/syntax"
)

// Process is an AsyncProcess: one method call running on its own
// goroutine, in an interpreter of its own, over copies of its arguments.
type Process struct {
	id     int
	method string
	done   chan struct{}
	mu     sync.Mutex
	result object.Value
	err    *object.Exception
	// awaited is set once something waited for the process, so await :all
	// and the end of the run know whose failure nobody saw.
	awaited bool
}

func (*Process) Class() *object.Class { return object.ProcessClass }

// wait blocks until the process is done and returns its result or the
// exception it raised.
func (p *Process) wait() (object.Value, *object.Exception) {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	p.awaited = true
	return p.result, p.err
}

func (p *Process) isDone() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// processList is every process a run started, in start order.
type processList struct {
	mu   sync.Mutex
	all  []*Process
	next int
}

func (l *processList) add(p *Process) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.next++
	p.id = l.next
	l.all = append(l.all, p)
}

// waitAll waits for every process and writes the failures nobody awaited to
// standard error.
func (l *processList) waitAll(in *Interp) {
	for i := 0; ; i++ {
		l.mu.Lock()
		if i >= len(l.all) {
			l.mu.Unlock()
			return
		}
		p := l.all[i]
		l.mu.Unlock()
		<-p.done
		p.mu.Lock()
		failed := p.err != nil && !p.awaited
		p.awaited = true
		exc := p.err
		p.mu.Unlock()
		if failed {
			in.Write(true, fmt.Sprintf("amber: process %d (%s) failed and was never awaited: %s (%s)\n", p.id, p.method, exc.Message, exc.Cls.QualifiedName()))
		}
	}
}

// evalAsync starts recv.method(*args) on a process and returns the process.
func (in *Interp) evalAsync(n *syntax.Async, env *Env, fr *Frame) (object.Value, error) {
	p, err := in.start(n.Recv, n.Method, n.Args, n.KwArgs, env, fr, n.Line())
	if err != nil {
		return nil, err
	}
	return p, nil
}

// evalAwait waits for a process, or starts one and waits for it.
func (in *Interp) evalAwait(n *syntax.Await, env *Env, fr *Frame) (object.Value, error) {
	var p *Process
	if n.Process != nil {
		v, err := in.eval(n.Process, env, fr)
		if err != nil {
			return nil, err
		}
		proc, ok := v.(*Process)
		if !ok {
			return nil, in.raise(object.TypeError, n.Line(), "await takes an AsyncProcess, not %s", typeName(v))
		}
		p = proc
	} else {
		proc, err := in.start(n.Recv, n.Method, n.Args, n.KwArgs, env, fr, n.Line())
		if err != nil {
			return nil, err
		}
		p = proc
	}
	result, exc := p.wait()
	if exc != nil {
		return nil, exc
	}
	return result, nil
}

// evalAwaitAll waits for every process this scope started and has not
// awaited, and returns their results in start order; when one failed it
// raises that failure after all have finished.
func (in *Interp) evalAwaitAll(env *Env, line int) (object.Value, error) {
	var pending []*Process
	for _, p := range methodScope(env).processes {
		p.mu.Lock()
		if !p.awaited {
			pending = append(pending, p)
		}
		p.mu.Unlock()
	}
	results := make([]object.Value, 0, len(pending))
	var first *object.Exception
	for _, p := range pending {
		v, exc := p.wait()
		if exc != nil {
			if first == nil {
				first = exc
			}
			continue
		}
		results = append(results, v)
	}
	if first != nil {
		return nil, first
	}
	return object.NewList(results...), nil
}

// methodScope is the scope a process belongs to: the method body's, or the
// top level's, that the current block scope sits in. A block is part of
// the method that runs it, so a method that fans out in a loop and then
// waits sees every process.
func methodScope(env *Env) *Env {
	for env.parent != nil {
		env = env.parent
	}
	return env
}

// start serialises the receiver, the method name and the arguments, and runs
// the call in a worker interpreter on a new goroutine.
func (in *Interp) start(recvNode, methodNode syntax.Node, argNodes []syntax.Node, kwNodes []syntax.MapEntry, env *Env, fr *Frame, line int) (*Process, error) {
	recv, err := in.eval(recvNode, env, fr)
	if err != nil {
		return nil, err
	}
	m, err := in.eval(methodNode, env, fr)
	if err != nil {
		return nil, err
	}
	var method string
	switch m := m.(type) {
	case object.Symbol:
		method = string(m)
	case object.String:
		method = string(m)
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
	recvData, err := encode(recv)
	if err != nil {
		return nil, err
	}
	argData := make([][]byte, len(args))
	for i, a := range args {
		if argData[i], err = encode(a); err != nil {
			return nil, err
		}
	}
	var kwData []byte
	if kw != nil {
		if kwData, err = encode(kw); err != nil {
			return nil, err
		}
	}
	p := &Process{method: method, done: make(chan struct{})}
	in.procs.add(p)
	scope := methodScope(env)
	scope.processes = append(scope.processes, p)
	worker := in.worker()
	go worker.run(p, recvData, method, argData, kwData)
	return p, nil
}

// worker makes the interpreter a process runs in: the same output, the
// same classes (immutable once bound) through a snapshot of the constants,
// its own scopes.
func (in *Interp) worker() *Interp {
	mainClass := object.NewClass("Object", object.ObjectClass)
	mainClass.Digest = "main"
	for _, m := range in.mainClass.Methods {
		mainClass.SetMethod(m)
	}
	w := &Interp{
		stdout:    in.stdout,
		stderr:    in.stderr,
		outMu:     in.outMu,
		file:      in.file,
		mainClass: mainClass,
		main:      &object.Instance{Cls: mainClass},
		top:       NewEnv(nil),
		consts:    make(map[string]object.Value, len(in.consts)),
		proto:     in.proto,
		procs:     in.procs,
		code:      in.code,
	}
	for name, v := range in.consts {
		w.consts[name] = v
	}
	return w
}

// run is the body of a process's goroutine: decode, call, keep the result.
func (w *Interp) run(p *Process, recvData []byte, method string, argData [][]byte, kwData []byte) {
	defer close(p.done)
	result, err := w.runCall(recvData, method, argData, kwData)
	p.mu.Lock()
	defer p.mu.Unlock()
	if err != nil {
		p.err = err
		return
	}
	p.result = result
}

func (w *Interp) runCall(recvData []byte, method string, argData [][]byte, kwData []byte) (object.Value, *object.Exception) {
	dec := w.Decoder()
	recv, err := dec.Decode(recvData)
	if err != nil {
		return nil, w.raise(object.TypeError, 0, "%s", err.Error()).(*object.Exception)
	}
	args := make([]object.Value, len(argData))
	for i, d := range argData {
		if args[i], err = dec.Decode(d); err != nil {
			return nil, w.raise(object.TypeError, 0, "%s", err.Error()).(*object.Exception)
		}
	}
	var kw *object.Map
	if kwData != nil {
		v, err := dec.Decode(kwData)
		if err != nil {
			return nil, w.raise(object.TypeError, 0, "%s", err.Error()).(*object.Exception)
		}
		// Keywords are symbols, which travelled as strings.
		if m, ok := v.(*object.Map); ok {
			b := object.NewMapBuilder()
			for i, k := range m.Keys() {
				if s, isStr := k.(object.String); isStr {
					k = object.Symbol(s)
				}
				b.Set(k, m.Values()[i])
			}
			kw = b.Map()
		}
	}
	fr := &Frame{Self: w.main, DefTarget: w.mainClass}
	result, callErr := w.call(fr, recv, method, args, kw, nil, false, 0)
	if callErr != nil {
		if exc, ok := callErr.(*object.Exception); ok {
			return nil, exc
		}
		if c, ok := callErr.(*control); ok {
			return nil, w.raise(object.LocalJumpError, c.line, "%s from a process", c.kind).(*object.Exception)
		}
		return nil, w.raise(object.AmberError, 0, "%s", callErr.Error()).(*object.Exception)
	}
	// The result crosses back to the awaiting scope, so it has to be a
	// value that could travel.
	if _, err := w.Encoder().Encode(result); err != nil {
		var ns *protogen.NotSerialisable
		if errors.As(err, &ns) {
			return nil, w.raise(object.TypeError, 0, "a process returned a %s, which cannot be passed back", ns.Kind).(*object.Exception)
		}
		return nil, w.raise(object.TypeError, 0, "%s", err.Error()).(*object.Exception)
	}
	// Processes the call started and never awaited are waited for at the
	// end of the run through the shared list.
	return result, nil
}

func init() {
	c := object.ProcessClass
	c.Define("done?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.BoolOf(self.(*Process).isDone()), nil
	})
	c.Define("value", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v, exc := self.(*Process).wait()
		if exc != nil {
			return nil, exc
		}
		return v, nil
	})
	c.Define("inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		p := self.(*Process)
		state := "running"
		if p.isDone() {
			state = "done"
		}
		return object.String(fmt.Sprintf("#<AsyncProcess %d %s %s>", p.id, p.method, state)), nil
	})
	c.Define("to_s", 0, c.Methods["inspect"].Native)
}
