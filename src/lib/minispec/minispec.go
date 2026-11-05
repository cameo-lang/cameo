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
// The counts and the describe names live in the run's state (Context.
// State), so examples an async call runs count too.
package minispec

import (
	"fmt"
	"strings"
	"sync"

	"github.com/cameo-lang/cameo/src/object"
)

var (
	minispecModule = object.NewModule("Minispec")
	// assertionClass is Minispec::Assertion, what a failed assertion
	// raises; it is an Exception and not a StandardError, so a bare rescue
	// in an example does not catch it. Skip is one too.
	assertionClass = object.NewClass("Assertion", object.ExceptionClass)
	skipClass      = object.NewClass("Skip", assertionClass)
)

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
	mu         sync.Mutex
	describes  []string
	runs       int
	assertions int
	failures   int
	errors     int
	skips      int
	results    []result
	reporting  bool
}

func stateOf(ctx object.Context) *state {
	return ctx.State("minispec", func() any { return &state{} }).(*state)
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
	object.ObjectClass.Consts["Minispec"] = object.ClassValue(m)
	for _, c := range []*object.Class{assertionClass, skipClass} {
		c.Lexical = m
		m.Consts[c.Name] = object.ClassValue(c)
	}
	o := object.ObjectClass
	o.Define("describe", -1, describe)
	o.Define("it", -1, it)
	o.Define("specify", -1, it)
	o.Define("skip", -1, skip)
	o.Define("flunk", -1, flunk)
	o.Define("pass", -1, pass)
	installAssertions(o)
}

func argError(ctx object.Context, given int, expected string) error {
	return ctx.Raise(object.ArgumentError, "wrong number of arguments (given %d, expected %s)", given, expected)
}

// describe runs its block with the name on the stack the examples inside
// are named by.
func describe(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if len(args) == 0 {
		return object.Value{}, argError(ctx, 0, "1+")
	}
	name, err := ctx.ToS(args[0])
	if err != nil {
		return object.Value{}, err
	}
	st := stateOf(ctx)
	st.mu.Lock()
	st.describes = append(st.describes, name)
	st.startReport(ctx)
	st.mu.Unlock()
	defer func() {
		st.mu.Lock()
		st.describes = st.describes[:len(st.describes)-1]
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
		return object.Value{}, argError(ctx, len(args), "0..1")
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
	full := strings.Join(st.describes, "::")
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
		return ctx.Call(object.Nil, "exit", object.Int(1))
	}
	return object.Nil, nil
}

func skip(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	msg, err := optionalMessage(ctx, args, 0, "Skipped, no message given")
	if err != nil {
		return object.Value{}, err
	}
	return object.Value{}, ctx.Raise(skipClass, "%s", msg)
}

func flunk(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	msg, err := optionalMessage(ctx, args, 0, "Epic Fail!")
	if err != nil {
		return object.Value{}, err
	}
	return check(ctx, false, msg, "")
}

func pass(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	if len(args) > 1 {
		return object.Value{}, argError(ctx, len(args), "0..1")
	}
	return check(ctx, true, "", "")
}

// optionalMessage is the message argument at index i, or fallback: a
// String, or a Proc whose result is the message.
func optionalMessage(ctx object.Context, args []object.Value, i int, fallback string) (string, error) {
	if len(args) > i+1 {
		return "", argError(ctx, len(args), fmt.Sprintf("%d..%d", i, i+1))
	}
	if len(args) <= i || args[i].IsNil() {
		return fallback, nil
	}
	v := args[i]
	if v.Kind() == object.KindProc {
		r, err := ctx.Yield(v)
		if err != nil {
			return "", err
		}
		v = r
	}
	return ctx.ToS(v)
}

// check counts an assertion and raises Minispec::Assertion with the
// message when it did not hold. The custom message a caller gave comes
// first, with a period and a line break after it, as minitest's does.
func check(ctx object.Context, ok bool, message, custom string) (object.Value, error) {
	st := stateOf(ctx)
	st.mu.Lock()
	st.assertions++
	st.mu.Unlock()
	if ok {
		return object.True, nil
	}
	if custom != "" {
		message = custom + ".\n" + message
	}
	return object.Value{}, ctx.Raise(assertionClass, "%s", message)
}

// inspect is minitest's mu_pp.
func inspect(ctx object.Context, v object.Value) (string, error) { return ctx.Inspect(v) }

// assertion defines name as an assertion of n values and an optional
// message, whose test returns whether it held and the message when it did
// not. refute is its negation, with the negated message.
func assertion(o *object.Class, name string, n int, test func(ctx object.Context, args []object.Value, negated bool) (bool, string, error)) {
	for _, negated := range []bool{false, true} {
		negated := negated
		fname := name
		if negated {
			fname = "refute" + strings.TrimPrefix(name, "assert")
		}
		o.Define(fname, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < n {
				return object.Value{}, argError(ctx, len(args), fmt.Sprintf("%d..%d", n, n+1))
			}
			custom, err := optionalMessage(ctx, args, n, "")
			if err != nil {
				return object.Value{}, err
			}
			ok, message, err := test(ctx, args[:n], negated)
			if err != nil {
				return object.Value{}, err
			}
			return check(ctx, ok != negated, message, custom)
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
	assertion(o, "assert", 1, func(ctx object.Context, args []object.Value, negated bool) (bool, string, error) {
		s, err := inspect(ctx, args[0])
		return args[0].Truthy(), fmt.Sprintf("Expected %s %s be truthy.", s, not(negated)), err
	})
	assertion(o, "assert_equal", 2, func(ctx object.Context, args []object.Value, negated bool) (bool, string, error) {
		exp, err := inspect(ctx, args[0])
		if err != nil {
			return false, "", err
		}
		act, err := inspect(ctx, args[1])
		if err != nil {
			return false, "", err
		}
		eq, err := ctx.Equal(args[0], args[1])
		if negated {
			return eq, fmt.Sprintf("Expected %s to not be equal to %s.", act, exp), err
		}
		return eq, fmt.Sprintf("Expected: %s\n  Actual: %s", exp, act), err
	})
	assertion(o, "assert_nil", 1, func(ctx object.Context, args []object.Value, negated bool) (bool, string, error) {
		s, err := inspect(ctx, args[0])
		return args[0].IsNil(), fmt.Sprintf("Expected %s %s be nil.", s, not(negated)), err
	})
	assertion(o, "assert_empty", 1, func(ctx object.Context, args []object.Value, negated bool) (bool, string, error) {
		s, err := inspect(ctx, args[0])
		if err != nil {
			return false, "", err
		}
		if !ctx.RespondsTo(args[0], "empty?") {
			return false, "", ctx.Raise(assertionClass, "Expected %s (%s) to respond to #empty?.", s, args[0].Class().QualifiedName())
		}
		empty, err := ctx.Call(args[0], "empty?")
		return empty.Truthy(), fmt.Sprintf("Expected %s %s be empty.", s, not(negated)), err
	})
	assertion(o, "assert_includes", 2, func(ctx object.Context, args []object.Value, negated bool) (bool, string, error) {
		coll, err := inspect(ctx, args[0])
		if err != nil {
			return false, "", err
		}
		obj, err := inspect(ctx, args[1])
		if err != nil {
			return false, "", err
		}
		if !ctx.RespondsTo(args[0], "include?") {
			return false, "", ctx.Raise(assertionClass, "Expected %s (%s) to respond to #include?.", coll, args[0].Class().QualifiedName())
		}
		in, err := ctx.Call(args[0], "include?", args[1])
		return in.Truthy(), fmt.Sprintf("Expected %s %s include %s.", coll, not(negated), obj), err
	})
	assertion(o, "assert_instance_of", 2, func(ctx object.Context, args []object.Value, negated bool) (bool, string, error) {
		cls, ok := args[0].AsClass()
		if !ok {
			return false, "", ctx.Raise(object.TypeError, "class or module required")
		}
		s, err := inspect(ctx, args[1])
		if negated {
			return args[1].Class() == cls, fmt.Sprintf("Expected %s to not be an instance of %s.", s, cls.QualifiedName()), err
		}
		return args[1].Class() == cls, fmt.Sprintf("Expected %s to be an instance of %s, not %s.", s, cls.QualifiedName(), args[1].Class().QualifiedName()), err
	})
	assertion(o, "assert_kind_of", 2, func(ctx object.Context, args []object.Value, negated bool) (bool, string, error) {
		cls, ok := args[0].AsClass()
		if !ok {
			return false, "", ctx.Raise(object.TypeError, "class or module required")
		}
		s, err := inspect(ctx, args[1])
		if negated {
			return args[1].Class().IsA(cls), fmt.Sprintf("Expected %s to not be a kind of %s.", s, cls.QualifiedName()), err
		}
		return args[1].Class().IsA(cls), fmt.Sprintf("Expected %s to be a kind of %s, not %s.", s, cls.QualifiedName(), args[1].Class().QualifiedName()), err
	})
	assertion(o, "assert_match", 2, func(ctx object.Context, args []object.Value, negated bool) (bool, string, error) {
		matcher := args[0]
		if _, isString := matcher.AsString(); isString {
			escaped, err := ctx.Call(object.ClassValue(object.RegexpClass), "escape", matcher)
			if err != nil {
				return false, "", err
			}
			if matcher, err = ctx.Call(object.ClassValue(object.RegexpClass), "new", escaped); err != nil {
				return false, "", err
			}
		}
		m, err := inspect(ctx, matcher)
		if err != nil {
			return false, "", err
		}
		s, err := inspect(ctx, args[1])
		if err != nil {
			return false, "", err
		}
		if !ctx.RespondsTo(matcher, "=~") {
			return false, "", ctx.Raise(assertionClass, "Expected %s (%s) to respond to #=~.", m, matcher.Class().QualifiedName())
		}
		got, err := ctx.Call(matcher, "=~", args[1])
		return got.Truthy(), fmt.Sprintf("Expected %s %s match %s.", m, not(negated), s), err
	})
	assertion(o, "assert_respond_to", 2, func(ctx object.Context, args []object.Value, negated bool) (bool, string, error) {
		s, err := inspect(ctx, args[0])
		if err != nil {
			return false, "", err
		}
		name, err := ctx.ToS(args[1])
		if negated {
			return ctx.RespondsTo(args[0], name), fmt.Sprintf("Expected %s to not respond to %s.", s, name), err
		}
		return ctx.RespondsTo(args[0], name), fmt.Sprintf("Expected %s (%s) to respond to #%s.", s, args[0].Class().QualifiedName(), name), err
	})
	assertion(o, "assert_predicate", 2, func(ctx object.Context, args []object.Value, negated bool) (bool, string, error) {
		s, err := inspect(ctx, args[0])
		if err != nil {
			return false, "", err
		}
		op, err := ctx.ToS(args[1])
		if err != nil {
			return false, "", err
		}
		got, err := ctx.Call(args[0], op)
		return got.Truthy(), fmt.Sprintf("Expected %s %s be %s.", s, not(negated), op), err
	})
	assertion(o, "assert_operator", 3, func(ctx object.Context, args []object.Value, negated bool) (bool, string, error) {
		s1, err := inspect(ctx, args[0])
		if err != nil {
			return false, "", err
		}
		op, err := ctx.ToS(args[1])
		if err != nil {
			return false, "", err
		}
		s2, err := inspect(ctx, args[2])
		if err != nil {
			return false, "", err
		}
		got, err := ctx.Call(args[0], op, args[2])
		return got.Truthy(), fmt.Sprintf("Expected %s %s be %s %s.", s1, not(negated), op, s2), err
	})
	// assert_in_delta takes its delta as an optional third argument, so it
	// is defined by hand: the message slot is the fourth.
	for _, negated := range []bool{false, true} {
		negated := negated
		name := "assert_in_delta"
		if negated {
			name = "refute_in_delta"
		}
		o.Define(name, -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) < 2 || len(args) > 4 {
				return object.Value{}, argError(ctx, len(args), "2..4")
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
			exp, err := ctx.ToS(args[0])
			if err != nil {
				return object.Value{}, err
			}
			act, err := ctx.ToS(args[1])
			if err != nil {
				return object.Value{}, err
			}
			n, err := ctx.ToS(diff)
			if err != nil {
				return object.Value{}, err
			}
			d, err := ctx.ToS(delta)
			if err != nil {
				return object.Value{}, err
			}
			return check(ctx, within.Truthy() != negated, fmt.Sprintf("Expected |%s - %s| (%s) %s be <= %s.", exp, act, n, not(negated), d), custom)
		})
	}
	o.Define("assert_raises", -1, assertRaises)
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
		return check(ctx, false, custom+expected+" expected but nothing was raised.", "")
	}
	exc, ok := err.(*object.Exception)
	if !ok {
		return object.Value{}, err
	}
	for _, c := range classes {
		if exc.Cls.IsA(c) {
			if _, err := check(ctx, true, "", ""); err != nil {
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
	return check(ctx, false, fmt.Sprintf("%s%s exception expected, not\nClass: <%s>\nMessage: <%q>", custom, expected, exc.Cls.QualifiedName(), exc.Message), "")
}
