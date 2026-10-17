package stdlib

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/radiospiel/amber/src/object"
)

// format implements the subset of Kernel#format amber supports: %s %d %i %f
// %e %g %x %o %b %c %% with flags, width and precision, and %<name>s with a
// map argument.
func format(ctx object.Context, f string, args []object.Value) (string, error) {
	var b strings.Builder
	next := 0
	var named *object.Map
	if len(args) == 1 {
		named, _ = args[0].AsMap()
	}
	for i := 0; i < len(f); i++ {
		c := f[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(f) {
			return "", ctx.Raise(object.ArgumentError, "incomplete format specifier; use %%%% (double %%) instead")
		}
		if f[i] == '%' {
			b.WriteByte('%')
			continue
		}
		start := i
		var arg object.Value
		if f[i] == '<' {
			end := strings.IndexByte(f[i:], '>')
			if end < 0 || named == nil {
				return "", ctx.Raise(object.ArgumentError, "one hash required")
			}
			name := f[i+1 : i+end]
			v, ok := named.Get(object.Symbol(name))
			if !ok {
				return "", ctx.Raise(object.KeyError, "key<%s> not found", name)
			}
			arg = v
			i += end + 1
			start = i
		}
		for i < len(f) && strings.IndexByte("-+ 0#.0123456789", f[i]) >= 0 {
			i++
		}
		if i >= len(f) {
			return "", ctx.Raise(object.ArgumentError, "malformed format string - %%%s", f[start:])
		}
		spec := f[start:i]
		verb := f[i]
		if arg.IsZero() {
			if next >= len(args) {
				return "", ctx.Raise(object.ArgumentError, "too few arguments")
			}
			arg = args[next]
			next++
		}
		s, err := formatOne(ctx, spec, verb, arg)
		if err != nil {
			return "", err
		}
		b.WriteString(s)
	}
	return b.String(), nil
}

func formatOne(ctx object.Context, spec string, verb byte, arg object.Value) (string, error) {
	switch verb {
	case 's':
		s, err := ctx.ToS(arg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%"+spec+"s", s), nil
	case 'p':
		s, err := ctx.Inspect(arg)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%"+spec+"s", s), nil
	case 'd', 'i', 'u':
		if arg.Kind() == object.KindBig {
			return fmt.Sprintf("%"+spec+"d", arg.Big()), nil
		}
		i, err := wantInt(ctx, arg, "")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%"+spec+"d", i), nil
	case 'x', 'X', 'o', 'b', 'B':
		v := verb
		if v == 'B' {
			v = 'b'
		}
		if arg.Kind() == object.KindBig {
			return fmt.Sprintf("%"+spec+string(v), arg.Big()), nil
		}
		i, err := wantInt(ctx, arg, "")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%"+spec+string(v), i), nil
	case 'f', 'e', 'E', 'g', 'G':
		var x float64
		switch arg.Kind() {
		case object.KindInt, object.KindBig:
			x, _ = arg.AsNumber()
		case object.KindFloat:
			v := arg.Float()
			x = float64(v)
		case object.KindString:
			v := arg.Str()
			p, err := strconv.ParseFloat(string(v), 64)
			if err != nil {
				return "", ctx.Raise(object.ArgumentError, "invalid value for Float(): %s", object.QuoteString(string(v)))
			}
			x = p
		default:
			return "", ctx.Raise(object.TypeError, "can't convert %s into Float", kindName(arg))
		}
		if verb == 'f' && !strings.Contains(spec, ".") {
			spec += ".6"
		}
		return fmt.Sprintf("%"+spec+string(verb), x), nil
	case 'c':
		switch arg.Kind() {
		case object.KindInt:
			v := arg.Int()
			return string(rune(v)), nil
		case object.KindString:
			v := arg.Str()
			if len(v) > 0 {
				return string([]rune(string(v))[0]), nil
			}
		}
		return "", ctx.Raise(object.ArgumentError, "%%c requires a character")
	}
	return "", ctx.Raise(object.ArgumentError, "malformed format string - %%%c", verb)
}

func parseInt(s string) (int64, bool) {
	i, ok := parseInteger(s)
	if !ok {
		return 0, false
	}
	return i.AsInt()
}

// parseInteger reads an integer literal, with an optional sign, base
// prefix and underscores, as Integer() does.
func parseInteger(s string) (object.Value, bool) {
	s = strings.ReplaceAll(s, "_", "")
	if i, err := strconv.ParseInt(s, 0, 64); err == nil {
		return object.Int(i), true
	}
	b, ok := new(big.Int).SetString(s, 0)
	if !ok {
		return object.Value{}, false
	}
	return object.BigInt(b), true
}

func parseFloat(s string) (float64, bool) {
	s = strings.ReplaceAll(s, "_", "")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// leadingInt reads the integer at the start of s the way String#to_i does:
// optional sign and digits, 0 when there are none.
func leadingInt(s string) object.Value {
	s = strings.TrimLeft(s, " \t\n")
	end := 0
	if end < len(s) && (s[end] == '-' || s[end] == '+') {
		end++
	}
	for end < len(s) && (s[end] >= '0' && s[end] <= '9' || s[end] == '_' && end > 0) {
		end++
	}
	digits := strings.ReplaceAll(s[:end], "_", "")
	if i, err := strconv.ParseInt(digits, 10, 64); err == nil {
		return object.Int(i)
	}
	if b, ok := new(big.Int).SetString(digits, 10); ok {
		return object.BigInt(b)
	}
	return object.Int(0)
}

// leadingFloat reads the float at the start of s the way String#to_f does.
func leadingFloat(s string) float64 {
	s = strings.TrimLeft(s, " \t\n")
	end := 0
	if end < len(s) && (s[end] == '-' || s[end] == '+') {
		end++
	}
	digits := func() {
		for end < len(s) && (s[end] >= '0' && s[end] <= '9' || s[end] == '_') {
			end++
		}
	}
	digits()
	if end < len(s) && s[end] == '.' && end+1 < len(s) && s[end+1] >= '0' && s[end+1] <= '9' {
		end++
		digits()
	}
	if end < len(s) && (s[end] == 'e' || s[end] == 'E') {
		save := end
		end++
		if end < len(s) && (s[end] == '-' || s[end] == '+') {
			end++
		}
		if end < len(s) && s[end] >= '0' && s[end] <= '9' {
			digits()
		} else {
			end = save
		}
	}
	f, _ := strconv.ParseFloat(strings.ReplaceAll(s[:end], "_", ""), 64)
	return f
}
