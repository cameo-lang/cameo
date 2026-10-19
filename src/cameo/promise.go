package cameo

import (
	"errors"
	"fmt"

	"github.com/cameo-lang/cameo/src/eval"
	"github.com/cameo-lang/cameo/src/object"
)

// Promise is a result the host hands a script before it has it: a host
// method returns the promise, which the script holds as an Integer and
// awaits, and the host settles it once, from any goroutine, when the work
// is done. It is how a script waits for I/O, which cameo leaves to the
// host:
//
//	web.Def("fetch", func(args []any, kwargs map[string]any) (any, error) {
//		promise := prog.Promise()
//		go func() {
//			body, err := fetch(args[0].(string))
//			if err != nil {
//				promise.Fail(err)
//				return
//			}
//			promise.Resolve(body)
//		}()
//		return promise, nil
//	})
type Promise struct{ p *eval.Promise }

// Promise makes a pending promise of the program.
func (p *Program) Promise() *Promise { return &Promise{p.in.NewPromise()} }

// ID is the Integer the script awaits.
func (p *Promise) ID() int64 { return p.p.ID() }

// ErrSettled is what Resolve and Fail return for a promise that is settled
// already.
var ErrSettled = errors.New("cameo: the promise is settled already")

// Resolve settles the promise with v, a Go value that converts as a host
// method's result does; await returns it. A value that does not convert
// raises a TypeError where the script awaits.
func (p *Promise) Resolve(v any) error {
	if !p.p.SettleLater(func(in *eval.Interp) (object.Value, error) {
		out, err := conv{in}.toValue(v)
		if err != nil {
			return object.Value{}, in.Raise(object.TypeError, "a promise was resolved with %s", err)
		}
		return out, nil
	}) {
		return ErrSettled
	}
	return nil
}

// Fail settles the promise with err, which await raises: an error an
// ExceptionClass made raises its class, any other a RuntimeError.
func (p *Promise) Fail(err error) error {
	if err == nil {
		err = fmt.Errorf("the promise failed")
	}
	if !p.p.SettleLater(func(in *eval.Interp) (object.Value, error) {
		var exc *Exception
		if errors.As(err, &exc) {
			return object.Value{}, in.Raise(exc.cls, "%s", exc.Message)
		}
		return object.Value{}, in.Raise(object.RuntimeError, "%s", err)
	}) {
		return ErrSettled
	}
	return nil
}
