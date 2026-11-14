// Package strscan is Ruby's strscan library, built into cameo:
// StringScanner, a handle that scans a String from a position with
// Regexps and Strings, as a tokeniser does.
//
// It follows strscan 3.1.2, the version Ruby 3.4 ships: scan, scan_until,
// skip, skip_until, check, check_until, match?, exist?, search_full and
// scan_full, the match record with matched, pre_match, post_match, [],
// captures, values_at and named_captures, getch, get_byte, scan_byte,
// peek, peek_byte, unscan, reset, terminate, pos, charpos, rest,
// beginning_of_line?, string=, concat, inspect and StringScanner::Error.
// A pattern's anchors hold at the scan position, as in Ruby; with
// fixed_anchor: true they hold at the string's start, found here by
// searching the whole string for a match at the position.
package strscan

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	class      = object.NewClass("StringScanner", object.ObjectClass)
	errorClass = object.NewClass("Error", object.StandardError)
)

// scanner is a StringScanner's state. mu guards it: async tasks may share
// a scanner.
type scanner struct {
	mu    sync.Mutex
	s     string
	pos   int
	prev  int // the position before the last match, for unscan
	fixed bool
	// The match record: the last match's span and groups, nil when the
	// last try failed.
	match *object.Match
	// literal is the last match of a String pattern, or of getch and its
	// kin: its span.
	lit    [2]int
	hasLit bool
	// lastRe is the last Regexp tried, matched or not, whose names
	// named_captures lists.
	lastRe *object.Regexp
}

func scannerOf(v object.Value) *scanner { return v.Instance().Message.(*scanner) }

// matched says whether the last try matched.
func (sc *scanner) matched() bool { return sc.match != nil || sc.hasLit }

// span is the last match's byte span.
func (sc *scanner) span() (int, int) {
	if sc.hasLit {
		return sc.lit[0], sc.lit[1]
	}
	return sc.match.Caps[0], sc.match.Caps[1]
}

// clear forgets the match record.
func (sc *scanner) clear() { sc.match, sc.hasLit = nil, false }

// advance records the n bytes at the position as the match, and moves
// past them: getch, get_byte and scan_byte.
func (sc *scanner) advance(n int) {
	sc.lit, sc.hasLit = [2]int{sc.pos, sc.pos + n}, true
	sc.prev, sc.pos = sc.pos, sc.pos+n
}

// try matches pattern at the position (anchored) or from it (until):
// the end of the match, or -1 when there is none. The match record is
// set either way.
func (sc *scanner) try(ctx object.Context, pattern object.Value, anchored bool) (int, error) {
	sc.clear()
	if sc.pos > len(sc.s) {
		return -1, nil
	}
	if str, ok := pattern.AsString(); ok {
		rest := sc.s[sc.pos:]
		var at int
		if anchored {
			if !strings.HasPrefix(rest, str) {
				return -1, nil
			}
			at = 0
		} else if at = strings.Index(rest, str); at < 0 {
			return -1, nil
		}
		sc.lit = [2]int{sc.pos + at, sc.pos + at + len(str)}
		sc.hasLit = true
		return sc.lit[1], nil
	}
	re, ok := pattern.AsRegexp()
	if !ok {
		return -1, ctx.Raise(object.TypeError, "no implicit conversion of %s into String", stdlib.KindName(pattern))
	}
	sc.lastRe = re
	var m *object.Match
	switch {
	case sc.fixed:
		// Anchors hold at the string's start, not at the position.
		m = re.MatchInString(sc.s, sc.pos, anchored)
	case anchored:
		m = re.MatchHere(sc.s, sc.pos)
	default:
		m = re.MatchAt(sc.s, sc.pos)
	}
	if m == nil {
		return -1, nil
	}
	sc.match = m
	return m.Caps[1], nil
}

// run is scan and kin: match, then advance when asked, and return the
// text or the length when asked, nil when the match fails.
func (sc *scanner) run(ctx object.Context, pattern object.Value, anchored, advance, returnString bool) (object.Value, error) {
	end, err := sc.try(ctx, pattern, anchored)
	if err != nil {
		return object.Value{}, err
	}
	if end < 0 {
		return object.Nil, nil
	}
	start := sc.pos
	if advance {
		sc.prev = sc.pos
		sc.pos = end
	}
	if returnString {
		return object.NewString(sc.s[start:end], object.UTF8), nil
	}
	return object.Int(end - start), nil
}

// group is the text of a group of the last match, nil for one that took
// no part; false for a name the pattern lacks.
func (sc *scanner) group(key object.Value) (object.Value, bool) {
	if !sc.matched() {
		return object.Nil, true
	}
	if sc.hasLit {
		if i, ok := key.AsInt(); ok && (i == 0 || i == -1) {
			return object.NewString(sc.s[sc.lit[0]:sc.lit[1]], object.UTF8), true
		}
		return object.Nil, true
	}
	i, isInt := key.AsInt()
	if !isInt {
		name, ok := key.AsSymbol()
		if !ok {
			name, ok = key.AsString()
		}
		if !ok {
			return object.Nil, true
		}
		idx := sc.match.NameIndex(name)
		if idx < 0 {
			return object.Nil, false
		}
		i = int64(idx)
	}
	if i < 0 {
		i += int64(sc.match.Groups() + 1)
	}
	if i < 0 || int(i) > sc.match.Groups() {
		return object.Nil, true
	}
	text, ok := sc.match.Group(int(i))
	if !ok {
		return object.Nil, true
	}
	return object.NewString(text, object.UTF8), true
}

// inspect is StringScanner#inspect: the position, five characters before
// and after it, or fin at the end.
func (sc *scanner) inspect() string {
	if sc.pos >= len(sc.s) {
		return "#<StringScanner fin>"
	}
	after := sc.s[sc.pos:]
	if len(after) > 5 {
		after = after[:5] + "..."
	}
	if sc.pos == 0 {
		return "#<StringScanner 0/" + strconv.Itoa(len(sc.s)) + " @ " + dump(after) + ">"
	}
	before := sc.s[:sc.pos]
	if len(before) > 5 {
		before = "..." + before[len(before)-5:]
	}
	return "#<StringScanner " + strconv.Itoa(sc.pos) + "/" + strconv.Itoa(len(sc.s)) + " " + dump(before) + " @ " + dump(after) + ">"
}

// dump quotes s as Ruby's String#dump does: printable ASCII as it is, the
// usual escapes, and every other byte as \xNN, a cut character's bytes
// included.
func dump(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\t':
			b.WriteString(`\t`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\f':
			b.WriteString(`\f`)
		case c == '\v':
			b.WriteString(`\v`)
		case c == '\a':
			b.WriteString(`\a`)
		case c == '\b':
			b.WriteString(`\b`)
		case c == 0x1b:
			b.WriteString(`\e`)
		case c == '#' && i+1 < len(s) && (s[i+1] == '{' || s[i+1] == '$' || s[i+1] == '@'):
			b.WriteString(`\#`)
		case c < 0x20 || c >= 0x7f:
			fmt.Fprintf(&b, `\x%02X`, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Install defines StringScanner. lib.Install calls it once.
func Install() {
	c := class
	object.DefineNative("StringScanner", object.ClassValue(c))
	c.Consts["Version"] = object.NewString("3.1.2", object.ASCII)
	c.Consts["Id"] = object.NewString("$Id$", object.ASCII)
	errorClass.Lexical = c
	c.Consts["Error"] = object.ClassValue(errorClass)
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..2")
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		sc := &scanner{s: s}
		if kw != nil {
			for _, k := range kw.Keys() {
				if name, _ := k.AsSymbol(); name != "fixed_anchor" {
					shown, _ := ctx.Inspect(k)
					return object.Value{}, ctx.Raise(object.ArgumentError, "unknown keyword: %s", shown)
				}
			}
			if v, ok := kw.Get(object.Symbol("fixed_anchor")); ok {
				sc.fixed = v.Truthy()
			}
		}
		// The instance is of the class new was called on: a subclass's.
		return object.InstanceValue(&object.Instance{Cls: self.Cls(), Message: sc}), nil
	})
	def := func(name string, arity int, f func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error)) {
		c.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			sc := scannerOf(self)
			sc.mu.Lock()
			defer sc.mu.Unlock()
			return f(ctx, sc, self, args)
		})
	}
	scanning := func(name string, anchored, advance, returnString bool) {
		def(name, 1, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
			return sc.run(ctx, args[0], anchored, advance, returnString)
		})
	}
	scanning("scan", true, true, true)
	scanning("scan_until", false, true, true)
	scanning("skip", true, true, false)
	scanning("skip_until", false, true, false)
	scanning("check", true, false, true)
	scanning("check_until", false, false, true)
	scanning("match?", true, false, false)
	scanning("exist?", false, false, false)
	full := func(name string, anchored bool) {
		def(name, 3, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
			return sc.run(ctx, args[0], anchored, args[1].Truthy(), args[2].Truthy())
		})
	}
	full("scan_full", true)
	full("search_full", false)

	def("getch", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		sc.clear()
		if sc.pos >= len(sc.s) {
			return object.Nil, nil
		}
		_, n := utf8.DecodeRuneInString(sc.s[sc.pos:])
		sc.advance(n)
		return object.NewString(sc.s[sc.lit[0]:sc.lit[1]], object.UTF8), nil
	})
	def("get_byte", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		sc.clear()
		if sc.pos >= len(sc.s) {
			return object.Nil, nil
		}
		sc.advance(1)
		return object.NewString(sc.s[sc.lit[0]:sc.lit[1]], object.UTF8), nil
	})
	def("scan_byte", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		sc.clear()
		if sc.pos >= len(sc.s) {
			return object.Nil, nil
		}
		b := sc.s[sc.pos]
		sc.advance(1)
		return object.Int(int64(b)), nil
	})
	def("peek_byte", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		if sc.pos >= len(sc.s) {
			return object.Nil, nil
		}
		return object.Int(int64(sc.s[sc.pos])), nil
	})
	def("peek", 1, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		n, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			return object.Value{}, ctx.Raise(object.ArgumentError, "negative string size (or size too big)")
		}
		if sc.pos >= len(sc.s) {
			return object.NewString("", object.ASCII), nil
		}
		return object.NewString(sc.s[sc.pos:sc.pos+int(min(n, int64(len(sc.s)-sc.pos)))], object.UTF8), nil
	})
	def("unscan", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		if !sc.matched() {
			return object.Value{}, ctx.Raise(errorClass, "unscan failed: previous match record not exist")
		}
		sc.pos = sc.prev
		sc.clear()
		return self, nil
	})
	def("reset", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		sc.pos = 0
		sc.clear()
		return self, nil
	})
	def("terminate", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		sc.pos = len(sc.s)
		sc.clear()
		return self, nil
	})
	c.Define("clear", 0, c.Methods["terminate"].Native)
	def("matched", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		if !sc.matched() {
			return object.Nil, nil
		}
		a, b := sc.span()
		return object.NewString(sc.s[a:b], object.UTF8), nil
	})
	def("matched?", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		return object.Bool(sc.matched()), nil
	})
	def("matched_size", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		if !sc.matched() {
			return object.Nil, nil
		}
		a, b := sc.span()
		return object.Int(b - a), nil
	})
	def("pre_match", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		if !sc.matched() {
			return object.Nil, nil
		}
		a, _ := sc.span()
		return object.NewString(sc.s[:a], object.UTF8), nil
	})
	def("post_match", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		if !sc.matched() {
			return object.Nil, nil
		}
		_, b := sc.span()
		return object.NewString(sc.s[b:], object.UTF8), nil
	})
	def("[]", 1, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		v, ok := sc.group(args[0])
		if !ok {
			name, _ := ctx.ToS(args[0])
			return object.Value{}, ctx.Raise(object.IndexError, "undefined group name reference: %s", name)
		}
		return v, nil
	})
	def("size", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		switch {
		case !sc.matched():
			return object.Nil, nil
		case sc.hasLit:
			return object.Int(1), nil
		}
		return object.Int(sc.match.Groups() + 1), nil
	})
	def("captures", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		if !sc.matched() {
			return object.Nil, nil
		}
		if sc.hasLit {
			return object.NewList(), nil
		}
		out := make([]object.Value, sc.match.Groups())
		for i := range out {
			out[i], _ = sc.group(object.Int(i + 1))
		}
		return object.NewList(out...), nil
	})
	def("values_at", -1, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		if !sc.matched() {
			return object.Nil, nil
		}
		out := make([]object.Value, len(args))
		for i, a := range args {
			out[i], _ = sc.group(a)
		}
		return object.NewList(out...), nil
	})
	def("named_captures", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		// The last Regexp's names, each nil unless the last match was its.
		m := object.NewMap()
		if sc.lastRe != nil {
			for _, name := range sc.lastRe.UniqueNames() {
				v := object.Nil
				if sc.match != nil {
					v, _ = sc.group(object.NewString(name, object.UTF8))
				}
				m = m.Put(object.NewString(name, object.UTF8), v)
			}
		}
		return object.MapValue(m), nil
	})
	def("pos", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		return object.Int(sc.pos), nil
	})
	c.Define("pointer", 0, c.Methods["pos"].Native)
	setPos := func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		n, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		if n < 0 {
			n += int64(len(sc.s))
		}
		if n < 0 || int(n) > len(sc.s) {
			return object.Value{}, ctx.Raise(object.RangeError, "index out of range")
		}
		sc.pos = int(n)
		return self, nil
	}
	def("pos=", 1, setPos)
	def("pointer=", 1, setPos)
	def("charpos", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		return object.Int(utf8.RuneCountInString(sc.s[:min(sc.pos, len(sc.s))])), nil
	})
	for _, name := range []string{"eos?", "empty?"} {
		def(name, 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
			return object.Bool(sc.pos >= len(sc.s)), nil
		})
	}
	def("rest?", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		return object.Bool(sc.pos < len(sc.s)), nil
	})
	def("rest", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		return object.NewString(sc.s[min(sc.pos, len(sc.s)):], object.UTF8), nil
	})
	def("rest_size", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		return object.Int(max(0, len(sc.s)-sc.pos)), nil
	})
	def("string", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		return object.NewString(sc.s, object.UTF8), nil
	})
	def("string=", 1, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		sc.s, sc.pos = s, 0
		sc.clear()
		return self, nil
	})
	for _, name := range []string{"concat", "<<"} {
		def(name, 1, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
			s, err := stdlib.WantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			sc.s += s
			return self, nil
		})
	}
	for _, name := range []string{"beginning_of_line?", "bol?"} {
		def(name, 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
			if sc.pos > len(sc.s) {
				return object.Nil, nil
			}
			return object.Bool(sc.pos == 0 || sc.s[sc.pos-1] == '\n'), nil
		})
	}
	def("fixed_anchor?", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		return object.Bool(sc.fixed), nil
	})
	def("inspect", 0, func(ctx object.Context, sc *scanner, self object.Value, args []object.Value) (object.Value, error) {
		return object.NewString(sc.inspect(), object.UTF8), nil
	})
}
