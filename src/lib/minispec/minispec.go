// Package minispec is cameo's test library, require "minispec": describe,
// it and skip, and minitest's assert_* and refute_* functions, with
// minitest's report when the script ends.
//
// Ruby's minitest discovers tests by method name and keeps its state in
// instance variables, neither of which cameo has, so minispec is cameo's
// own, with minitest's names and messages (minitest 5.25) where they fit.
// An example runs when it is called: a dot, F, E or S is printed for it,
// and the report at the end lists every failure, error and skip with its
// line, then counts them. A script with a failure or an error ends with
// status 1. There is no before, after or let: a local assigned in the
// describe block is what its examples share.
//
// The counts live in the run's state (Context.State), so examples an
// async call runs count too; the describe names are each interpreter's,
// so workers running examples at once do not mix theirs. The functions
// are the methods of two modules, Minispec::DSL (describe, it, specify) and
// Minispec::Assertions (the assertions, skip, flunk and pass), as minitest's
// are Minitest::Spec::DSL's and Minitest::Assertions': require "minispec"
// includes both at the top level of the script that requires it, and a
// class that wants the assertions in its methods includes
// Minispec::Assertions, as a Ruby class includes Minitest::Assertions.
package minispec

import (
	"fmt"
	"strings"
	"sync"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	minispecModule = object.NewModule("Minispec")
	// assertionClass is Minispec::Assertion, what a failed assertion
	// raises; it is an Exception and not a StandardError, so a bare rescue
	// in an example does not catch it. Skip is one too.
	assertionClass = object.NewClass("Assertion", object.ExceptionClass)
	skipClass      = object.NewClass("Skip", assertionClass)
	// dslModule is Minispec::DSL: describe, it and specify.
	dslModule = object.NewModule("DSL")
	// assertionsModule is Minispec::Assertions: the assertions, skip,
	// flunk and pass.
	assertionsModule = object.NewModule("Assertions")
)

// TopLevel are the modules require "minispec" includes at the top level of
// the script: what a script calls describe, it and the assertions on.
var TopLevel = []*object.Class{dslModule, assertionsModule}

// result is one failed, erroring or skipped example, as the report lists
// it.
type result struct {
	kind    string // "Failure", "Error" or "Skipped"
	name    string
	line    int
	message string
}

// state is a run's: the describe blocks running, the counts and the
// results to report.
type state struct {
	mu sync.Mutex
	// describes are the describe blocks running, by the interpreter
	// running them: a worker's examples are named by the worker's.
	describes  map[object.Context][]string
	runs       int
	assertions int
	failures   int
	errors     int
	skips      int
	results    []result
	reporting  bool
}

func stateOf(ctx object.Context) *state {
	return ctx.State("minispec", func() any { return &state{describes: map[object.Context][]string{}} }).(*state)
}

// startReport registers the report with at_exit, once, on the first
// describe or it; the caller holds st.mu. A script with a describe and no
// example reports 0 runs, as minitest does.
func (st *state) startReport(ctx object.Context) {
	if st.reporting {
		return
	}
	st.reporting = true
	ctx.AtExit(ctx.Block(func([]object.Value) (object.Value, error) { return report(ctx) }))
}

// Install defines Minispec, describe, it, skip and the assertions.
// lib.Install calls it once.
func Install() {
	m := minispecModule
	object.DefineNative("Minispec", object.ClassValue(m))
	for _, c := range []*object.Class{assertionClass, skipClass, dslModule, assertionsModule} {
		c.Lexical = m
		m.Consts[c.Name] = object.ClassValue(c)
	}
	define(dslModule, "describe", describe)
	define(dslModule, "it", it)
	define(dslModule, "specify", it)
	a := assertionsModule
	define(a, "skip", skip)
	define(a, "flunk", flunk)
	define(a, "pass", pass)
	installAssertions(a)
}

// define defines name in the module o, answering only in a script that
// required minispec: the method tables are the process's, the require a
// script's, so a class that includes the module in a script that did not
// ask for the library sees no `assert`.
func define(o *object.Class, name string, f object.NativeFn) {
	o.Define(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if !ctx.Required("minispec") {
			return object.Value{}, ctx.Raise(object.NoMethodError, "undefined method '%s' for %s: require \"minispec\" first", name, describeSelf(self))
		}
		return f(ctx, self, args, kw, block)
	})
}

// describeSelf names a receiver as a NoMethodError does.
func describeSelf(self object.Value) string {
	switch {
	case self.IsNil():
		return "nil"
	case self.Kind() == object.KindClass:
		return self.Class().QualifiedName()
	}
	return "an instance of " + self.Class().QualifiedName()
}

// describe runs its block with the name on the stack the examples inside
// are named by.
func describe(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if len(args) == 0 {
		return object.Value{}, stdlib.ArgError(ctx, 0, "1+")
	}
	name, err := ctx.ToS(args[0])
	if err != nil {
		return object.Value{}, err
	}
	st := stateOf(ctx)
	st.mu.Lock()
	st.describes[ctx] = append(st.describes[ctx], name)
	st.startReport(ctx)
	st.mu.Unlock()
	defer func() {
		st.mu.Lock()
		st.describes[ctx] = st.describes[ctx][:len(st.describes[ctx])-1]
		st.mu.Unlock()
	}()
	if block.IsZero() {
		return object.Nil, nil
	}
	if _, err := ctx.Yield(block); err != nil {
		return object.Value{}, err
	}
	return object.Nil, nil
}

// it runs one example and prints its mark. Without a block it is skipped,
// as minitest's is.
func it(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if len(args) > 1 {
		return object.Value{}, stdlib.ArgError(ctx, len(args), "0..1")
	}
	name := ""
	if len(args) == 1 {
		s, err := ctx.ToS(args[0])
		if err != nil {
			return object.Value{}, err
		}
		name = s
	}
	st := stateOf(ctx)
	st.mu.Lock()
	full := strings.Join(st.describes[ctx], "::")
	if full != "" {
		full += "#"
	}
	full += name
	st.runs++
	st.startReport(ctx)
	st.mu.Unlock()
	var err error
	if block.IsZero() {
		err = ctx.Raise(skipClass, "(no tests defined)")
	} else {
		_, err = ctx.Yield(block)
	}
	mark := "."
	if err != nil {
		exc, ok := err.(*object.Exception)
		if !ok || exc.Cls.IsA(object.SystemExit) {
			return object.Value{}, err
		}
		r := result{name: full, line: exc.Line, message: exc.Message}
		st.mu.Lock()
		switch {
		case exc.Cls.IsA(skipClass):
			r.kind, mark = "Skipped", "S"
			st.skips++
		case exc.Cls.IsA(assertionClass):
			r.kind, mark = "Failure", "F"
			st.failures++
		default:
			r.kind, mark = "Error", "E"
			r.message = exc.Cls.QualifiedName() + ": " + exc.Message
			st.errors++
		}
		st.results = append(st.results, r)
		st.mu.Unlock()
	}
	if err := ctx.Write(false, mark); err != nil {
		return object.Value{}, err
	}
	return object.Nil, nil
}

// report prints the results and the counts, and ends the script with
// status 1 when an example failed or raised.
func report(ctx object.Context) (object.Value, error) {
	st := stateOf(ctx)
	st.mu.Lock()
	defer st.mu.Unlock()
	var b strings.Builder
	b.WriteString("\n\n")
	for i, r := range st.results {
		where := ctx.File()
		if r.line > 0 {
			where = fmt.Sprintf("%s:%d", where, r.line)
		}
		fmt.Fprintf(&b, "  %d) %s:\n%s [%s]:\n%s\n\n", i+1, r.kind, r.name, where, r.message)
	}
	fmt.Fprintf(&b, "%d runs, %d assertions, %d failures, %d errors, %d skips\n", st.runs, st.assertions, st.failures, st.errors, st.skips)
	if err := ctx.Write(false, b.String()); err != nil {
		return object.Value{}, err
	}
	if st.failures+st.errors > 0 {
		return object.Value{}, object.NewSystemExit(1, "exit")
	}
	return object.Nil, nil
}

func skip(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	msg, err := optionalMessage(ctx, args, 0, "Skipped, no message given")
	if err != nil {
		return object.Value{}, err
	}
	text, err := msg()
	if err != nil {
		return object.Value{}, err
	}
	return object.Value{}, ctx.Raise(skipClass, "%s", text)
}

func flunk(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	msg, err := optionalMessage(ctx, args, 0, "Epic Fail!")
	if err != nil {
		return object.Value{}, err
	}
	return check(ctx, false, msg, nil)
}

func pass(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if len(args) > 1 {
		return object.Value{}, stdlib.ArgError(ctx, len(args), "0..1")
	}
	return check(ctx, true, nil, nil)
}

// message is the text of a failure, made only when the assertion failed:
// minitest builds its messages, and calls a message Proc, on failure
// alone, and a passing assertion is the common case.
type message func() (string, error)

// text is a message of fixed text.
func text(format string, args ...any) message {
	s := fmt.Sprintf(format, args...)
	return func() (string, error) { return s, nil }
}

// optionalMessage is the message argument at index i, or fallback: a
// String, or a Proc whose result is the message, called on failure.
func optionalMessage(ctx object.Context, args []object.Value, i int, fallback string) (message, error) {
	if len(args) > i+1 {
		return nil, stdlib.ArgError(ctx, len(args), fmt.Sprintf("%d..%d", i, i+1))
	}
	if len(args) <= i || args[i].IsNil() {
		return text("%s", fallback), nil
	}
	v := args[i]
	return func() (string, error) {
		if v.Kind() == object.KindProc {
			r, err := ctx.Yield(v)
			if err != nil {
				return "", err
			}
			return ctx.ToS(r)
		}
		return ctx.ToS(v)
	}, nil
}

// check counts an assertion and raises Minispec::Assertion with the
// message when it did not hold. The custom message a caller gave comes
// first, with a period and a line break after it, as minitest's does.
func check(ctx object.Context, ok bool, msg, custom message) (object.Value, error) {
	st := stateOf(ctx)
	st.mu.Lock()
	st.assertions++
	st.mu.Unlock()
	if ok {
		return object.True, nil
	}
	s, err := msg()
	if err != nil {
		return object.Value{}, err
	}
	if custom != nil {
		c, err := custom()
		if err != nil {
			return object.Value{}, err
		}
		if c != "" {
			s = c + ".\n" + s
		}
	}
	return object.Value{}, ctx.Raise(assertionClass, "%s", s)
}

// inspected is the inspect of v, as minitest's mu_pp, made on failure.
func inspected(ctx object.Context, v object.Value) func() string {
	return func() string {
		s, err := ctx.Inspect(v)
		if err != nil {
			return "?"
		}
		return s
	}
}

// assertion defines name as an assertion of n values and an optional
// message, whose test returns whether it held and the message for when it
// did not. refute is its negation, with the negated message.
func assertion(o *object.Class, name string, n int, test func(ctx object.Context, args []object.Value, negated bool) (bool, message, error)) {
	for _, negated := range []bool{false, true} {
		negated := negated
		fname := name
		if negated {
			fname = "refute" + strings.TrimPrefix(name, "assert")
		}
		define(o, fname, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < n {
				return object.Value{}, stdlib.ArgError(ctx, len(args), fmt.Sprintf("%d..%d", n, n+1))
			}
			custom, err := optionalMessage(ctx, args, n, "")
			if err != nil {
				return object.Value{}, err
			}
			ok, msg, err := test(ctx, args[:n], negated)
			if err != nil {
				return object.Value{}, err
			}
			return check(ctx, ok != negated, msg, custom)
		})
	}
}

// not is "to" or "to not", as the message of a refutation reads.
func not(negated bool) string {
	if negated {
		return "to not"
	}
	return "to"
}

func installAssertions(o *object.Class) {
	assertion(o, "assert", 1, func(ctx object.Context, args []object.Value, negated bool) (bool, message, error) {
		v := inspected(ctx, args[0])
		return args[0].Truthy(), func() (string, error) { return fmt.Sprintf("Expected %s %s be truthy.", v(), not(negated)), nil }, nil
	})
	assertion(o, "assert_equal", 2, func(ctx object.Context, args []object.Value, negated bool) (bool, message, error) {
		eq, err := ctx.Equal(args[0], args[1])
		exp, act := inspected(ctx, args[0]), inspected(ctx, args[1])
		if negated {
			return eq, func() (string, error) { return fmt.Sprintf("Expected %s to not be equal to %s.", act(), exp()), nil }, err
		}
		return eq, func() (string, error) { return fmt.Sprintf("Expected: %s\n  Actual: %s", exp(), act()), nil }, err
	})
	assertion(o, "assert_nil", 1, func(ctx object.Context, args []object.Value, negated bool) (bool, message, error) {
		v := inspected(ctx, args[0])
		return args[0].IsNil(), func() (string, error) { return fmt.Sprintf("Expected %s %s be nil.", v(), not(negated)), nil }, nil
	})
	assertion(o, "assert_empty", 1, func(ctx object.Context, args []object.Value, negated bool) (bool, message, error) {
		v := inspected(ctx, args[0])
		if !ctx.RespondsTo(args[0], "empty?") {
			return false, nil, ctx.Raise(assertionClass, "Expected %s (%s) to respond to #empty?.", v(), args[0].Class().QualifiedName())
		}
		empty, err := ctx.Call(args[0], "empty?")
		return empty.Truthy(), func() (string, error) { return fmt.Sprintf("Expected %s %s be empty.", v(), not(negated)), nil }, err
	})
	assertion(o, "assert_includes", 2, func(ctx object.Context, args []object.Value, negated bool) (bool, message, error) {
		coll, obj := inspected(ctx, args[0]), inspected(ctx, args[1])
		if !ctx.RespondsTo(args[0], "include?") {
			return false, nil, ctx.Raise(assertionClass, "Expected %s (%s) to respond to #include?.", coll(), args[0].Class().QualifiedName())
		}
		in, err := ctx.Call(args[0], "include?", args[1])
		return in.Truthy(), func() (string, error) {
			return fmt.Sprintf("Expected %s %s include %s.", coll(), not(negated), obj()), nil
		}, err
	})
	assertion(o, "assert_instance_of", 2, func(ctx object.Context, args []object.Value, negated bool) (bool, message, error) {
		cls, ok := args[0].AsClass()
		if !ok {
			return false, nil, ctx.Raise(object.TypeError, "class or module required")
		}
		v := inspected(ctx, args[1])
		if negated {
			return args[1].Class() == cls, func() (string, error) {
				return fmt.Sprintf("Expected %s to not be an instance of %s.", v(), cls.QualifiedName()), nil
			}, nil
		}
		return args[1].Class() == cls, func() (string, error) {
			return fmt.Sprintf("Expected %s to be an instance of %s, not %s.", v(), cls.QualifiedName(), args[1].Class().QualifiedName()), nil
		}, nil
	})
	assertion(o, "assert_kind_of", 2, func(ctx object.Context, args []object.Value, negated bool) (bool, message, error) {
		cls, ok := args[0].AsClass()
		if !ok {
			return false, nil, ctx.Raise(object.TypeError, "class or module required")
		}
		v := inspected(ctx, args[1])
		if negated {
			return args[1].Class().IsA(cls), func() (string, error) {
				return fmt.Sprintf("Expected %s to not be a kind of %s.", v(), cls.QualifiedName()), nil
			}, nil
		}
		return args[1].Class().IsA(cls), func() (string, error) {
			return fmt.Sprintf("Expected %s to be a kind of %s, not %s.", v(), cls.QualifiedName(), args[1].Class().QualifiedName()), nil
		}, nil
	})
	assertion(o, "assert_match", 2, func(ctx object.Context, args []object.Value, negated bool) (bool, message, error) {
		matcher := args[0]
		if _, isString := matcher.AsString(); isString {
			escaped, err := ctx.Call(object.ClassValue(object.RegexpClass), "escape", matcher)
			if err != nil {
				return false, nil, err
			}
			if matcher, err = ctx.Call(object.ClassValue(object.RegexpClass), "new", escaped); err != nil {
				return false, nil, err
			}
		}
		m, v := inspected(ctx, matcher), inspected(ctx, args[1])
		if !ctx.RespondsTo(matcher, "=~") {
			return false, nil, ctx.Raise(assertionClass, "Expected %s (%s) to respond to #=~.", m(), matcher.Class().QualifiedName())
		}
		got, err := ctx.Call(matcher, "=~", args[1])
		return got.Truthy(), func() (string, error) { return fmt.Sprintf("Expected %s %s match %s.", m(), not(negated), v()), nil }, err
	})
	assertion(o, "assert_respond_to", 2, func(ctx object.Context, args []object.Value, negated bool) (bool, message, error) {
		name, err := ctx.ToS(args[1])
		if err != nil {
			return false, nil, err
		}
		v := inspected(ctx, args[0])
		if negated {
			return ctx.RespondsTo(args[0], name), func() (string, error) { return fmt.Sprintf("Expected %s to not respond to %s.", v(), name), nil }, nil
		}
		return ctx.RespondsTo(args[0], name), func() (string, error) {
			return fmt.Sprintf("Expected %s (%s) to respond to #%s.", v(), args[0].Class().QualifiedName(), name), nil
		}, nil
	})
	predicate := func(ctx object.Context, args []object.Value, negated bool) (bool, message, error) {
		op, err := ctx.ToS(args[1])
		if err != nil {
			return false, nil, err
		}
		got, err := ctx.Call(args[0], op)
		v := inspected(ctx, args[0])
		return got.Truthy(), func() (string, error) { return fmt.Sprintf("Expected %s %s be %s.", v(), not(negated), op), nil }, err
	}
	assertion(o, "assert_predicate", 2, predicate)
	// assert_operator takes two values and the operator between them, or
	// one value and a predicate, as minitest's does.
	for _, negated := range []bool{false, true} {
		negated := negated
		name := "assert_operator"
		if negated {
			name = "refute_operator"
		}
		define(o, name, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < 2 || len(args) > 4 {
				return object.Value{}, stdlib.ArgError(ctx, len(args), "2..4")
			}
			if len(args) == 2 {
				ok, msg, err := predicate(ctx, args, negated)
				if err != nil {
					return object.Value{}, err
				}
				return check(ctx, ok != negated, msg, nil)
			}
			custom, err := optionalMessage(ctx, args, 3, "")
			if err != nil {
				return object.Value{}, err
			}
			op, err := ctx.ToS(args[1])
			if err != nil {
				return object.Value{}, err
			}
			got, err := ctx.Call(args[0], op, args[2])
			if err != nil {
				return object.Value{}, err
			}
			s1, s2 := inspected(ctx, args[0]), inspected(ctx, args[2])
			return check(ctx, got.Truthy() != negated, func() (string, error) {
				return fmt.Sprintf("Expected %s %s be %s %s.", s1(), not(negated), op, s2()), nil
			}, custom)
		})
	}
	// assert_in_delta takes its delta as an optional third argument, so it
	// is defined by hand: the message slot is the fourth.
	for _, negated := range []bool{false, true} {
		negated := negated
		name := "assert_in_delta"
		if negated {
			name = "refute_in_delta"
		}
		define(o, name, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < 2 || len(args) > 4 {
				return object.Value{}, stdlib.ArgError(ctx, len(args), "2..4")
			}
			delta := object.Float(0.001)
			if len(args) >= 3 {
				delta = args[2]
			}
			custom, err := optionalMessage(ctx, args, 3, "")
			if err != nil {
				return object.Value{}, err
			}
			diff, err := ctx.Call(args[0], "-", args[1])
			if err != nil {
				return object.Value{}, err
			}
			if diff, err = ctx.Call(diff, "abs"); err != nil {
				return object.Value{}, err
			}
			within, err := ctx.Call(diff, "<=", delta)
			if err != nil {
				return object.Value{}, err
			}
			return check(ctx, within.Truthy() != negated, func() (string, error) {
				parts := make([]string, 4)
				for i, v := range []object.Value{args[0], args[1], diff, delta} {
					s, err := ctx.ToS(v)
					if err != nil {
						return "", err
					}
					parts[i] = s
				}
				return fmt.Sprintf("Expected |%s - %s| (%s) %s be <= %s.", parts[0], parts[1], parts[2], not(negated), parts[3]), nil
			}, custom)
		})
	}
	define(o, "assert_raises", assertRaises)
}

// assertRaises runs its block and returns the exception it raised, of one
// of the classes given (StandardError when none). An Assertion or a
// SystemExit of another class is raised again.
func assertRaises(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if block.IsZero() {
		return object.Value{}, ctx.Raise(object.ArgumentError, "assert_raises requires a block to capture errors")
	}
	custom := ""
	if len(args) > 0 {
		if s, ok := args[len(args)-1].AsString(); ok {
			custom = s + ".\n"
			args = args[:len(args)-1]
		}
	}
	classes := make([]*object.Class, 0, len(args))
	for _, a := range args {
		cls, ok := a.AsClass()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "class or module required")
		}
		classes = append(classes, cls)
	}
	if len(classes) == 0 {
		classes = append(classes, object.StandardError)
	}
	expected := classes[0].QualifiedName()
	if len(classes) > 1 {
		names := make([]string, len(classes))
		for i, c := range classes {
			names[i] = c.QualifiedName()
		}
		expected = "[" + strings.Join(names, ", ") + "]"
	}
	_, err := ctx.Yield(block)
	if err == nil {
		return check(ctx, false, text("%s%s expected but nothing was raised.", custom, expected), nil)
	}
	exc, ok := err.(*object.Exception)
	if !ok {
		return object.Value{}, err
	}
	for _, c := range classes {
		if exc.Cls.IsA(c) {
			if _, err := check(ctx, true, nil, nil); err != nil {
				return object.Value{}, err
			}
			return object.ExceptionValue(exc), nil
		}
	}
	// An Assertion, a Skip or a SystemExit the caller did not expect is
	// raised again, after the expected classes were tried, in the order of
	// minitest's rescue clauses.
	if exc.Cls.IsA(assertionClass) || exc.Cls.IsA(object.SystemExit) {
		return object.Value{}, err
	}
	return check(ctx, false, text("%s%s exception expected, not\nClass: <%s>\nMessage: <%q>", custom, expected, exc.Cls.QualifiedName(), exc.Message), nil)
}
