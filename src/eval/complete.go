package eval

import (
	"regexp"
	"sort"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/syntax"
)

// keywords are the words a line may start with that are not names.
var keywords = []string{
	"alias", "and", "begin", "break", "case", "class", "def", "defined?", "do",
	"else", "elsif", "end", "ensure", "false", "for", "if", "in", "module",
	"next", "nil", "not", "or", "redo", "rescue", "retry", "return", "self",
	"super", "then", "true", "undef", "unless", "until", "when", "while", "yield",
}

// wordAtEnd is the name being typed at the end of a line.
var wordAtEnd = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*[?!]?$|$`)

// receiverAtEnd is what a line ends in before a dot whose methods can be
// listed without running anything but a lookup or a literal: a variable, a
// constant or a path of them, a number, a string or a symbol.
var receiverAtEnd = regexp.MustCompile(`(?:[A-Za-z_][A-Za-z0-9_]*(?:::[A-Z][A-Za-z0-9_]*)*|-?\d+(?:\.\d+)?|"[^"\\#]*"|'[^'\\]*'|:[A-Za-z_][A-Za-z0-9_]*[?!]?)$`)

// name is a method name a line can type: no operators.
var name = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*[?!=]?$`)

// Completions returns the name being typed at the end of before, a line
// of cameo irb up to the cursor, and the names it can be completed to at
// the top level of the run, sorted: after a dot, the methods of the
// receiver, when it is a variable, a constant or a literal; else the
// variables, the methods of the top level, the constants and the keywords.
// It runs nothing but the lookup of a constant or a literal.
func (in *Interp) Completions(before string) (word string, names []string) {
	word = wordAtEnd.FindString(before)
	rest := before[:len(before)-len(word)]
	var candidates []string
	if strings.HasSuffix(rest, ".") && !strings.HasSuffix(rest, "..") {
		recv := receiverAtEnd.FindString(strings.TrimSuffix(rest, "."))
		if recv == "" {
			return word, nil
		}
		v, ok := in.completionReceiver(recv)
		if !ok {
			return word, nil
		}
		candidates = methodsOf(v)
	} else {
		in.top.each(func(n string, _ object.Value) { candidates = append(candidates, n) })
		for _, k := range in.mainClass.Ancestors() {
			candidates = append(candidates, k.MethodNames()...)
		}
		for n := range in.consts {
			candidates = append(candidates, n)
		}
		for n := range object.ObjectClass.Consts {
			candidates = append(candidates, n)
		}
		candidates = append(candidates, keywords...)
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		if strings.HasPrefix(c, word) && name.MatchString(c) && !seen[c] {
			seen[c] = true
			names = append(names, c)
		}
	}
	sort.Strings(names)
	return word, names
}

// completionReceiver returns the value of recv: a variable's, without
// calling a method of that name, or a constant's or a literal's.
func (in *Interp) completionReceiver(recv string) (object.Value, bool) {
	if c := recv[0]; c == '_' || (c >= 'a' && c <= 'z') {
		var found object.Value
		ok := false
		in.top.each(func(n string, v object.Value) {
			if n == recv {
				found, ok = v, true
			}
		})
		return found, ok
	}
	prog, err := syntax.Parse("(completion)", recv)
	if err != nil {
		return object.Value{}, false
	}
	v, err := in.runStatements(prog.Body, in.top, in.topFrame())
	return v, err == nil
}

// methodsOf returns the names of the methods v answers: a class's or a
// module's own, and those of its class, or an instance's class's.
func methodsOf(v object.Value) []string {
	var names []string
	if c, ok := v.AsClass(); ok {
		for k := c; k != nil; k = k.Super {
			for n := range k.SingletonMethods {
				names = append(names, n)
			}
			for _, e := range k.Extends {
				for _, a := range e.Ancestors() {
					names = append(names, a.MethodNames()...)
				}
			}
		}
	}
	for _, k := range v.Class().Ancestors() {
		names = append(names, k.MethodNames()...)
	}
	return names
}
