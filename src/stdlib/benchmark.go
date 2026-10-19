package stdlib

import (
	"regexp"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
)

// Benchmark is Ruby 3.4's benchmark 0.4.0, in Go: it reads the process's
// CPU times and the monotonic clock itself, so a script times its own code
// without the interpreter's start in the result. Tms#add! is left out:
// it changes its receiver, which a cameo value cannot.

var (
	benchmarkModule = object.NewModule("Benchmark")
	tmsClass        = object.NewClass("Tms", object.ObjectClass)
	// BenchmarkTmsClass is Benchmark::Tms, which p and puts render with
	// BenchmarkTmsInspect and BenchmarkTmsToS.
	BenchmarkTmsClass = tmsClass
	// reportClass is the x of Benchmark.bm { |x| ... }, jobClass that of
	// bmbm; each keeps what its report calls collect in Instance.Message.
	reportClass = object.NewClass("Report", object.ObjectClass)
	jobClass    = object.NewClass("Job", object.ObjectClass)
)

const (
	benchmarkCaption = "      user     system      total        real\n"
	benchmarkFormat  = "%10.6u %10.6y %10.6t %10.6r\n"
)

// The fields of a Tms instance, in Tms.new's order, and its total.
const (
	tmsUtime = iota
	tmsStime
	tmsCutime
	tmsCstime
	tmsReal
	tmsLabel
	tmsTotal
)

var tmsReaders = []string{"utime", "stime", "cutime", "cstime", "real", "label", "total"}

// reportState is a Benchmark::Report's: the label width so far and the
// times its report calls measured.
type reportState struct {
	width  int
	format object.Value
	list   []object.Value
}

// jobState is a Benchmark::Job's: the label width so far and the blocks
// bmbm runs twice.
type jobState struct {
	width  int
	labels []string
	blocks []object.Value
}

func installBenchmark() {
	m := benchmarkModule
	object.ObjectClass.Consts["Benchmark"] = object.ClassValue(m)
	m.Consts["VERSION"] = object.String("0.4.0")
	m.Consts["BENCHMARK_VERSION"] = object.String("2002-04-25")
	m.Consts["CAPTION"] = object.String(benchmarkCaption)
	m.Consts["FORMAT"] = object.String(benchmarkFormat)
	for _, c := range []*object.Class{tmsClass, reportClass, jobClass} {
		c.Lexical = m
		m.Consts[c.Name] = object.ClassValue(c)
	}
	tmsClass.Consts["CAPTION"] = object.String(benchmarkCaption)
	tmsClass.Consts["FORMAT"] = object.String(benchmarkFormat)

	// The module functions are instance methods too, for a class that
	// includes Benchmark.
	define := func(name string, f fn) {
		m.DefineSingleton(name, -1, f)
		m.Define(name, -1, f)
	}
	define("measure", func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		label := object.String("")
		if len(args) == 1 {
			label = args[0]
		}
		return benchmarkMeasure(ctx, label, block, "measure")
	})
	define("realtime", func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 0 {
			return object.Value{}, argError(ctx, len(args), "0")
		}
		if err := needBlock(ctx, block, "yield"); err != nil {
			return object.Value{}, err
		}
		start := time.Now()
		if _, err := ctx.Yield(block); err != nil {
			return object.Value{}, err
		}
		return object.Float(time.Since(start).Seconds()), nil
	})
	define("benchmark", func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		caption, width, format := object.String(""), object.Nil, object.Nil
		for i, a := range args {
			switch i {
			case 0:
				caption = a
			case 1:
				width = a
			case 2:
				format = a
			}
		}
		var labels []object.Value
		if len(args) > 3 {
			labels = args[3:]
		}
		return benchmarkRun(ctx, caption, width, format, labels, block)
	})
	define("bm", func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		width := object.Int(0)
		var labels []object.Value
		if len(args) > 0 {
			width, labels = args[0], args[1:]
		}
		return benchmarkRun(ctx, object.String(benchmarkCaption), width, object.String(benchmarkFormat), labels, block)
	})
	define("bmbm", func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 1 {
			return object.Value{}, argError(ctx, len(args), "0..1")
		}
		width := int64(0)
		if len(args) == 1 {
			w, err := wantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			width = w
		}
		return benchmarkBmbm(ctx, int(width), block)
	})

	installTms()
	installReports()
}

// cpuTimes returns the user and system CPU seconds of this process and of
// its finished children; see rusage_unix.go.
type cpuTimes struct{ utime, stime, cutime, cstime float64 }

// benchmarkMeasure runs block and returns the Tms of its run.
func benchmarkMeasure(ctx object.Context, label, block object.Value, name string) (object.Value, error) {
	if err := needBlock(ctx, block, "yield"); err != nil {
		return object.Value{}, err
	}
	t0, r0 := readCPUTimes(), time.Now()
	if _, err := ctx.Yield(block); err != nil {
		return object.Value{}, err
	}
	t1, real := readCPUTimes(), time.Since(r0).Seconds()
	return newTms(ctx, []object.Value{
		object.Float(t1.utime - t0.utime),
		object.Float(t1.stime - t0.stime),
		object.Float(t1.cutime - t0.cutime),
		object.Float(t1.cstime - t0.cstime),
		object.Float(real),
	}, label)
}

// newTms is Tms.new(utime, stime, cutime, cstime, real, label): the times
// it is not given are 0.0, and total is their sum.
func newTms(ctx object.Context, times []object.Value, label object.Value) (object.Value, error) {
	fields := make([]object.Value, 7)
	for i := tmsUtime; i <= tmsReal; i++ {
		fields[i] = object.Float(0)
		if i < len(times) {
			fields[i] = times[i]
		}
	}
	s, err := ctx.ToS(object.OrNil(label))
	if err != nil {
		return object.Value{}, err
	}
	fields[tmsLabel] = object.String(s)
	total := fields[tmsUtime]
	for _, i := range []int{tmsStime, tmsCutime, tmsCstime} {
		if total, err = ctx.Call(total, "+", fields[i]); err != nil {
			return object.Value{}, err
		}
	}
	fields[tmsTotal] = total
	return object.InstanceValue(&object.Instance{Cls: tmsClass, Fields: fields}), nil
}

func tmsFields(self object.Value) []object.Value { return self.Instance().Fields }

// tmsDirectives are Tms#format's directives in the order it replaces them,
// each with the field it prints and the conversion it prints it with.
var tmsDirectives = []struct {
	re    *regexp.Regexp
	field int
	conv  string
}{
	{regexp.MustCompile(`(%[-+.\d]*)n`), tmsLabel, "s"},
	{regexp.MustCompile(`(%[-+.\d]*)u`), tmsUtime, "f"},
	{regexp.MustCompile(`(%[-+.\d]*)y`), tmsStime, "f"},
	{regexp.MustCompile(`(%[-+.\d]*)U`), tmsCutime, "f"},
	{regexp.MustCompile(`(%[-+.\d]*)Y`), tmsCstime, "f"},
	{regexp.MustCompile(`(%[-+.\d]*)t`), tmsTotal, "f"},
	{regexp.MustCompile(`(%[-+.\d]*)r`), tmsReal, "f"},
}

// tmsFormat is Tms#format: format, or FORMAT when it is nil, with the
// directives replaced by the times, and then, when format was given,
// String#% of args.
func tmsFormat(ctx object.Context, self, format object.Value, args []object.Value) (string, error) {
	str := benchmarkFormat
	if !format.IsZero() && !format.IsNil() {
		s, err := wantString(ctx, format)
		if err != nil {
			return "", err
		}
		str = s
	}
	fields := tmsFields(self)
	var failed error
	for _, d := range tmsDirectives {
		str = d.re.ReplaceAllStringFunc(str, func(m string) string {
			if failed != nil {
				return m
			}
			spec := d.re.FindStringSubmatch(m)[1] + d.conv
			out, err := ctx.Call(object.String(spec), "%", fields[d.field])
			if err != nil {
				failed = err
				return m
			}
			if d.field == tmsReal {
				return "(" + out.Str() + ")"
			}
			return out.Str()
		})
	}
	if failed != nil {
		return "", failed
	}
	if format.IsZero() || format.IsNil() {
		return str, nil
	}
	out, err := ctx.Call(object.String(str), "%", object.NewList(args...))
	if err != nil {
		return "", err
	}
	return out.Str(), nil
}

// BenchmarkTmsToS is Tms#to_s: its times in FORMAT.
func BenchmarkTmsToS(ctx object.Context, t object.Value) (string, error) {
	return tmsFormat(ctx, t, object.Nil, nil)
}

// BenchmarkTmsInspect is Tms#inspect: Ruby's instance variables, without
// the address Ruby shows and a cameo value has not.
func BenchmarkTmsInspect(ctx object.Context, t object.Value) (string, error) {
	var b strings.Builder
	b.WriteString("#<Benchmark::Tms")
	for i, name := range tmsReaders {
		s, err := ctx.Inspect(tmsFields(t)[i])
		if err != nil {
			return "", err
		}
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(" @" + name + "=" + s)
	}
	return b.String() + ">", nil
}

func installTms() {
	c := tmsClass
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 6 {
			return object.Value{}, argError(ctx, len(args), "0..6")
		}
		label := object.Nil
		if len(args) == 6 {
			label = args[5]
		}
		times := args
		if len(times) > 5 {
			times = times[:5]
		}
		return newTms(ctx, times, label)
	})
	for i, name := range tmsReaders {
		def(c, name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return tmsFields(self)[i], nil
		})
	}
	def(c, "format", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		format := object.Nil
		if len(args) > 0 {
			format, args = args[0], args[1:]
		}
		s, err := tmsFormat(ctx, self, format, args)
		if err != nil {
			return object.Value{}, err
		}
		return object.String(s), nil
	})
	def(c, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := BenchmarkTmsToS(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		return object.String(s), nil
	})
	def(c, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		s, err := BenchmarkTmsInspect(ctx, self)
		if err != nil {
			return object.Value{}, err
		}
		return object.String(s), nil
	})
	def(c, "to_a", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		f := tmsFields(self)
		return object.NewList(f[tmsLabel], f[tmsUtime], f[tmsStime], f[tmsCutime], f[tmsCstime], f[tmsReal]), nil
	})
	def(c, "to_h", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		f := tmsFields(self)
		b := object.NewMapBuilder()
		for _, i := range []int{tmsLabel, tmsUtime, tmsStime, tmsCutime, tmsCstime, tmsReal} {
			b.Set(object.Symbol(tmsReaders[i]), f[i])
		}
		return object.MapValue(b.Map()), nil
	})
	for _, op := range []string{"+", "-", "*", "/"} {
		def(c, op, 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return tmsMemberwise(ctx, op, self, args[0])
		})
	}
	def(c, "add", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		t, err := benchmarkMeasure(ctx, object.String(""), block, "add")
		if err != nil {
			return object.Value{}, err
		}
		return tmsMemberwise(ctx, "+", self, t)
	})
}

// tmsMemberwise applies op to each time of a Tms and the same time of x, or
// x itself when it is not a Tms. The result has no label.
func tmsMemberwise(ctx object.Context, op string, self, x object.Value) (object.Value, error) {
	a := tmsFields(self)
	other, isTms := x.AsInstance()
	isTms = isTms && other.Cls == tmsClass
	times := make([]object.Value, 5)
	for i := range times {
		rhs := x
		if isTms {
			rhs = other.Fields[i]
		}
		v, err := ctx.Call(a[i], op, rhs)
		if err != nil {
			return object.Value{}, err
		}
		times[i] = v
	}
	return newTms(ctx, times, object.Nil)
}

func installReports() {
	report := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		st := self.Instance().Message.(*reportState)
		label := object.String("")
		if len(args) > 0 {
			label = args[0]
		}
		s, err := ctx.ToS(label)
		if err != nil {
			return object.Value{}, err
		}
		st.width = max(st.width, utf8.RuneCountInString(s))
		t, err := benchmarkMeasure(ctx, label, block, "report")
		if err != nil {
			return object.Value{}, err
		}
		st.list = append(st.list, t)
		return t, nil
	}
	def(reportClass, "report", -1, report)
	def(reportClass, "item", -1, report)
	def(reportClass, "width", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(self.Instance().Message.(*reportState).width)), nil
	})
	def(reportClass, "list", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.NewList(self.Instance().Message.(*reportState).list...), nil
	})

	item := func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if block.IsZero() {
			return object.Value{}, ctx.Raise(object.ArgumentError, "no block")
		}
		st := self.Instance().Message.(*jobState)
		label := object.String("")
		if len(args) > 0 {
			label = args[0]
		}
		s, err := ctx.ToS(label)
		if err != nil {
			return object.Value{}, err
		}
		st.width = max(st.width, utf8.RuneCountInString(s))
		st.labels = append(st.labels, s)
		st.blocks = append(st.blocks, block)
		return self, nil
	}
	def(jobClass, "report", -1, item)
	def(jobClass, "item", -1, item)
	def(jobClass, "width", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(self.Instance().Message.(*jobState).width)), nil
	})
}

// benchmarkRun is Benchmark.benchmark: it yields a Report, then writes the
// caption, a line per report call, and a line per Tms in what the block
// returned when that is an Array, labelled by labels in turn.
func benchmarkRun(ctx object.Context, caption, widthArg, format object.Value, labels []object.Value, block object.Value) (object.Value, error) {
	if err := needBlock(ctx, block, "yield"); err != nil {
		return object.Value{}, err
	}
	labelWidth := int64(0)
	if !widthArg.IsNil() {
		w, err := wantInt(ctx, widthArg, "")
		if err != nil {
			return object.Value{}, err
		}
		labelWidth = w
	}
	labelWidth++
	if format.IsNil() {
		format = object.String(benchmarkFormat)
	}
	st := &reportState{width: int(labelWidth), format: format}
	report := object.InstanceValue(&object.Instance{Cls: reportClass, Message: st})
	results, err := ctx.Yield(block, report)
	if err != nil {
		return object.Value{}, err
	}
	var out strings.Builder
	captionStr, err := wantString(ctx, caption)
	if err != nil {
		return object.Value{}, err
	}
	if captionStr != "" {
		out.WriteString(strings.Repeat(" ", st.width) + captionStr)
	}
	for _, t := range st.list {
		line, err := tmsFormat(ctx, t, st.format, []object.Value{format})
		if err != nil {
			return object.Value{}, err
		}
		out.WriteString(padRight(tmsFields(t)[tmsLabel].Str(), st.width, " ") + line)
	}
	if l, ok := results.AsList(); ok {
		for _, r := range l.Elems {
			inst, isInst := r.AsInstance()
			if !isInst || inst.Cls != tmsClass {
				continue
			}
			label := inst.Fields[tmsLabel].Str()
			if len(labels) > 0 {
				s, err := ctx.ToS(labels[0])
				if err != nil {
					return object.Value{}, err
				}
				label, labels = s, labels[1:]
			}
			line, err := tmsFormat(ctx, r, format, nil)
			if err != nil {
				return object.Value{}, err
			}
			out.WriteString(padRight(label, int(labelWidth), " ") + line)
		}
	}
	if err := ctx.Write(false, out.String()); err != nil {
		return object.Value{}, err
	}
	return object.NewList(st.list...), nil
}

// benchmarkBmbm is Benchmark.bmbm: it runs every block a first time as a
// rehearsal, collects garbage, and runs them again for the times it
// returns, writing each line as its block finishes.
func benchmarkBmbm(ctx object.Context, width int, block object.Value) (object.Value, error) {
	if err := needBlock(ctx, block, "yield"); err != nil {
		return object.Value{}, err
	}
	st := &jobState{width: width}
	job := object.InstanceValue(&object.Instance{Cls: jobClass, Message: st})
	if _, err := ctx.Yield(block, job); err != nil {
		return object.Value{}, err
	}
	width = st.width + 1
	captionLen := utf8.RuneCountInString(benchmarkCaption)
	write := func(s string) error { return ctx.Write(false, s) }
	if err := write(padRight("Rehearsal ", width+captionLen, "-") + "\n"); err != nil {
		return object.Value{}, err
	}
	sum, err := newTms(ctx, nil, object.Nil)
	if err != nil {
		return object.Value{}, err
	}
	for i, b := range st.blocks {
		if err := write(padRight(st.labels[i], width, " ")); err != nil {
			return object.Value{}, err
		}
		res, err := benchmarkMeasure(ctx, object.String(""), b, "bmbm")
		if err != nil {
			return object.Value{}, err
		}
		line, err := tmsFormat(ctx, res, object.Nil, nil)
		if err != nil {
			return object.Value{}, err
		}
		if err := write(line); err != nil {
			return object.Value{}, err
		}
		if sum, err = tmsMemberwise(ctx, "+", sum, res); err != nil {
			return object.Value{}, err
		}
	}
	ets, err := tmsFormat(ctx, sum, object.String("total: %tsec"), nil)
	if err != nil {
		return object.Value{}, err
	}
	if err := write(padLeft(" "+ets+"\n\n", width+captionLen+2, "-") + strings.Repeat(" ", width) + benchmarkCaption); err != nil {
		return object.Value{}, err
	}
	results := make([]object.Value, len(st.blocks))
	for i, b := range st.blocks {
		runtime.GC()
		if err := write(padRight(st.labels[i], width, " ")); err != nil {
			return object.Value{}, err
		}
		res, err := benchmarkMeasure(ctx, object.String(st.labels[i]), b, "bmbm")
		if err != nil {
			return object.Value{}, err
		}
		line, err := tmsFormat(ctx, res, object.Nil, nil)
		if err != nil {
			return object.Value{}, err
		}
		if err := write(line); err != nil {
			return object.Value{}, err
		}
		results[i] = res
	}
	return object.NewList(results...), nil
}

// padRight and padLeft are String#ljust and #rjust with a pad string of
// one character.
func padRight(s string, width int, pad string) string {
	if n := utf8.RuneCountInString(s); n < width {
		return s + strings.Repeat(pad, width-n)
	}
	return s
}

func padLeft(s string, width int, pad string) string {
	if n := utf8.RuneCountInString(s); n < width {
		return strings.Repeat(pad, width-n) + s
	}
	return s
}
