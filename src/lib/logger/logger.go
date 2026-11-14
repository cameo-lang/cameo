// Package logger is Ruby's logger library, built into cameo: Logger on
// the script's standard output or error, with the levels and their
// predicates, progname, formatter, datetime_format and Logger::Formatter.
//
// It follows logger 1.6.4, the version Ruby 3.4 ships. A Logger is a
// handle on a stream, so level=, progname=, formatter= and
// datetime_format= change it for every holder, as they do in Ruby. The
// stream is STDOUT, STDERR or nil; a file name, log rotation and
// Logger::LogDevice are left out, since cameo writes no files.
package logger

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	class          = object.NewClass("Logger", object.ObjectClass)
	formatterClass = object.NewClass("Formatter", object.ObjectClass)
	severity       = object.NewModule("Severity")
	errorClass     = object.NewClass("Error", object.RuntimeError)
)

var labels = []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL", "ANY"}

const (
	debug = iota
	info
	warn
	errorLevel
	fatal
	unknown
)

// logger is a Logger's state, shared by every holder of the handle.
type logger struct {
	// stream is 0 for none, 1 for stdout, 2 for stderr.
	stream         int
	level          int
	progname       object.Value
	formatter      object.Value // a Proc, or Nil for the default
	datetimeFormat string
}

func loggerOf(v object.Value) *logger { return v.Instance().Message.(*logger) }

// formatter is a Logger::Formatter's state: its datetime_format.
type formatter struct{ datetimeFormat string }

func formatterOf(v object.Value) *formatter { return v.Instance().Message.(*formatter) }

// levelOf reads a level: an Integer, or a name as a Symbol or String.
func levelOf(ctx object.Context, v object.Value) (int, error) {
	if n, ok := v.AsInt(); ok {
		return int(n), nil
	}
	name, ok := v.AsSymbol()
	if !ok {
		name, ok = v.AsString()
	}
	if ok {
		switch strings.ToLower(name) {
		case "debug":
			return debug, nil
		case "info":
			return info, nil
		case "warn":
			return warn, nil
		case "error":
			return errorLevel, nil
		case "fatal":
			return fatal, nil
		case "unknown":
			return unknown, nil
		}
		return 0, ctx.Raise(object.ArgumentError, "invalid log level: %s", name)
	}
	return 0, ctx.Raise(object.ArgumentError, "invalid log level: %s", stdlib.KindName(v))
}

func label(sev int) string {
	if sev < 0 || sev >= len(labels) {
		return labels[unknown]
	}
	return labels[sev]
}

// msg2str is Logger::Formatter#msg2str: a String as it is, an exception
// with its class, anything else inspected.
func msg2str(ctx object.Context, msg object.Value) (string, error) {
	if msg.IsString() {
		return msg.Str(), nil
	}
	if e, ok := msg.AsException(); ok {
		return e.Message + " (" + e.Cls.QualifiedName() + ")\n", nil
	}
	return ctx.Inspect(msg)
}

// format is Logger::Formatter#call.
func format(ctx object.Context, datetimeFormat string, sev string, t time.Time, progname, msg object.Value) (string, error) {
	text, err := msg2str(ctx, msg)
	if err != nil {
		return "", err
	}
	prog := ""
	if !progname.IsNil() {
		if prog, err = ctx.ToS(progname); err != nil {
			return "", err
		}
	}
	df := datetimeFormat
	if df == "" {
		df = "%Y-%m-%dT%H:%M:%S.%6N"
	}
	padded := strings.Repeat(" ", max(0, 5-len(sev))) + sev
	return sev[:1] + ", [" + stdlib.Strftime(t, df) + " #" + strconv.Itoa(os.Getpid()) + "] " + padded + " -- " + prog + ": " + text + "\n", nil
}

// write sends a line to the logger's stream.
func (l *logger) write(ctx object.Context, s string) error {
	switch l.stream {
	case 1:
		return ctx.Write(false, s)
	case 2:
		return ctx.Write(true, s)
	}
	return nil
}

// add is Logger#add: the message, or the block's, or the progname as the
// message, under the severity, when the level admits it.
func (l *logger) add(ctx object.Context, sev int, message, progname, block object.Value) error {
	if l.stream == 0 || sev < l.level {
		return nil
	}
	if message.IsZero() || message.IsNil() {
		if !block.IsZero() {
			v, err := ctx.Yield(block)
			if err != nil {
				return err
			}
			message = v
		} else {
			message, progname = progname, object.Value{}
		}
	}
	if progname.IsZero() || progname.IsNil() {
		progname = l.progname
	}
	if message.IsZero() {
		message = object.Nil
	}
	now := time.Now()
	var line string
	if l.formatter.IsZero() || l.formatter.IsNil() {
		var err error
		if line, err = format(ctx, l.datetimeFormat, label(sev), now, progname, message); err != nil {
			return err
		}
	} else {
		v, err := ctx.Yield(l.formatter, object.NewString(label(sev), object.UTF8), object.TimeValue(now), progname, message)
		if err != nil {
			return err
		}
		if line, err = ctx.ToS(v); err != nil {
			return err
		}
	}
	return l.write(ctx, line)
}

// streamOf reads the device argument: STDOUT, STDERR or nil.
func streamOf(ctx object.Context, v object.Value) (int, error) {
	if v.IsNil() {
		return 0, nil
	}
	if inst, ok := v.AsInstance(); ok && inst.Cls == stdlib.IOClass {
		if inst.Fields[0].Str() == "stderr" {
			return 2, nil
		}
		return 1, nil
	}
	if v.IsString() {
		return 0, ctx.Raise(object.ArgumentError, "Logger.new(%q): cameo writes no files; log to STDOUT, STDERR or nil", v.Str())
	}
	return 0, ctx.Raise(object.TypeError, "Logger.new takes STDOUT, STDERR or nil, not %s", stdlib.KindName(v))
}

// Install defines Logger. lib.Install calls it once.
func Install() {
	c := class
	object.DefineNative("Logger", object.ClassValue(c))
	c.Consts["VERSION"] = object.NewString("1.6.4", object.ASCII)
	for i, name := range []string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL", "UNKNOWN"} {
		c.Consts[name] = object.Int(i)
		severity.Consts[name] = object.Int(i)
	}
	sevLabels := make([]object.Value, len(labels))
	for i, l := range labels {
		sevLabels[i] = object.NewString(l, object.UTF8)
	}
	c.Consts["SEV_LABEL"] = object.NewList(sevLabels...)
	for _, k := range []*object.Class{formatterClass, severity, errorClass} {
		k.Lexical = c
		c.Consts[k.Name] = object.ClassValue(k)
	}
	c.Include(severity)

	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 3 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..3")
		}
		stream, err := streamOf(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		l := &logger{stream: stream, progname: object.Nil, formatter: object.Nil}
		if kw != nil {
			for i, k := range kw.Keys() {
				name, _ := k.AsSymbol()
				v := kw.Values()[i]
				switch name {
				case "level":
					if l.level, err = levelOf(ctx, v); err != nil {
						return object.Value{}, err
					}
				case "progname":
					l.progname = v
				case "formatter":
					l.formatter = v
				case "datetime_format":
					if !v.IsNil() {
						if l.datetimeFormat, err = stdlib.WantString(ctx, v); err != nil {
							return object.Value{}, err
						}
					}
				case "shift_age", "shift_size", "binmode", "shift_period_suffix", "reraise_write_errors", "skip_header":
				default:
					inspect, _ := ctx.Inspect(k)
					return object.Value{}, ctx.Raise(object.ArgumentError, "unknown keyword: %s", inspect)
				}
			}
		}
		return object.InstanceValue(&object.Instance{Cls: c, Message: l}), nil
	})
	def := func(name string, arity int, f func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)) {
		c.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(ctx, loggerOf(self), self, args, kw, block)
		})
	}
	def("level", 0, func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(l.level), nil
	})
	def("level=", 1, func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		n, err := levelOf(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		l.level = n
		return self, nil
	})
	for i, name := range []string{"debug", "info", "warn", "error", "fatal", "unknown"} {
		sev := i
		def(name+"?", 0, func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Bool(l.level <= sev), nil
		})
		def(name, -1, func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if len(args) > 1 {
				return object.Value{}, stdlib.ArgError(ctx, len(args), "0..1")
			}
			message := object.Value{}
			if len(args) == 1 {
				message = args[0]
			}
			// The one argument is the message, or the progname when the
			// block gives the message.
			if !block.IsZero() {
				return object.True, l.add(ctx, sev, object.Value{}, message, block)
			}
			return object.True, l.add(ctx, sev, message, object.Value{}, object.Value{})
		})
	}
	add := func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 3 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "1..3")
		}
		sev := unknown
		if !args[0].IsNil() {
			var err error
			if sev, err = levelOf(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		message, progname := object.Value{}, object.Value{}
		if len(args) > 1 {
			message = args[1]
		}
		if len(args) > 2 {
			progname = args[2]
		}
		return object.True, l.add(ctx, sev, message, progname, block)
	}
	def("add", -1, add)
	def("log", -1, add)
	def("<<", 1, func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := ctx.ToS(args[0])
		if err != nil {
			return object.Value{}, err
		}
		return self, l.write(ctx, s)
	})
	def("progname", 0, func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return l.progname, nil
	})
	def("progname=", 1, func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		l.progname = args[0]
		return self, nil
	})
	def("formatter", 0, func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return l.formatter, nil
	})
	def("formatter=", 1, func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		l.formatter = args[0]
		return self, nil
	})
	def("datetime_format", 0, func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if l.datetimeFormat == "" {
			return object.Nil, nil
		}
		return object.NewString(l.datetimeFormat, object.UTF8), nil
	})
	def("datetime_format=", 1, func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if args[0].IsNil() {
			l.datetimeFormat = ""
			return self, nil
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		l.datetimeFormat = s
		return self, nil
	})
	for _, name := range []string{"close", "reopen"} {
		def(name, -1, func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Nil, nil
		})
	}
	def("inspect", 0, func(ctx object.Context, l *logger, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewString("#<Logger level:"+strings.ToLower(label(l.level))+">", object.UTF8), nil
	})

	// Logger::Formatter, the default format as a value a script can call
	// or give a datetime_format.
	f := formatterClass
	f.DefineSingleton("new", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.InstanceValue(&object.Instance{Cls: f, Message: &formatter{}}), nil
	})
	f.Define("call", 4, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		sev, err := ctx.ToS(args[0])
		if err != nil {
			return object.Value{}, err
		}
		t, ok := args[1].AsTime()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "the time is a Time, not %s", stdlib.KindName(args[1]))
		}
		s, err := format(ctx, formatterOf(self).datetimeFormat, sev, t, args[2], args[3])
		if err != nil {
			return object.Value{}, err
		}
		return object.NewString(s, object.UTF8), nil
	})
	f.Define("datetime_format", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if formatterOf(self).datetimeFormat == "" {
			return object.Nil, nil
		}
		return object.NewString(formatterOf(self).datetimeFormat, object.UTF8), nil
	})
	f.Define("datetime_format=", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		formatterOf(self).datetimeFormat = s
		return self, nil
	})
}
