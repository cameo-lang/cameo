package stdlib

import (
	"time"

	"github.com/cameo-lang/cameo/src/object"
)

// monotonicBase is the origin of CLOCK_MONOTONIC: a time read with Go's
// monotonic clock, so the differences of two readings do not jump with the
// wall clock.
var monotonicBase = time.Now()

// Clock ids, Linux's numbers as Ruby reports them.
const (
	clockRealtime  = 0
	clockMonotonic = 1
)

// installProcess defines Process.clock_gettime, the clock a benchmark
// reads, for the realtime and monotonic clocks. The rest of Process stays
// out: cameo starts no processes and reads no environment.
func installProcess() {
	p := object.ProcessModule
	p.Consts["CLOCK_REALTIME"] = object.Int(clockRealtime)
	p.Consts["CLOCK_MONOTONIC"] = object.Int(clockMonotonic)
	p.DefineSingleton("clock_gettime", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) < 1 || len(args) > 2 {
			return object.Value{}, argError(ctx, len(args), "1..2")
		}
		id, err := wantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		var ns int64
		switch id {
		case clockRealtime:
			ns = time.Now().UnixNano()
		case clockMonotonic:
			ns = int64(time.Since(monotonicBase))
		default:
			return object.Value{}, ctx.Raise(object.ArgumentError, "Invalid argument - clock_gettime(%d)", id)
		}
		unit := "float_second"
		if len(args) == 2 && !args[1].IsNil() {
			s, ok := args[1].AsSymbol()
			if !ok {
				s, err := ctx.ToS(args[1])
				if err != nil {
					return object.Value{}, err
				}
				return object.Value{}, ctx.Raise(object.ArgumentError, "unexpected unit: %s", s)
			}
			unit = s
		}
		switch unit {
		case "float_second":
			return object.Float(float64(ns) / 1e9), nil
		case "float_millisecond":
			return object.Float(float64(ns) / 1e6), nil
		case "float_microsecond":
			return object.Float(float64(ns) / 1e3), nil
		case "second":
			return object.Int(ns / 1e9), nil
		case "millisecond":
			return object.Int(ns / 1e6), nil
		case "microsecond":
			return object.Int(ns / 1e3), nil
		case "nanosecond":
			return object.Int(ns), nil
		}
		return object.Value{}, ctx.Raise(object.ArgumentError, "unexpected unit: %s", unit)
	})
}
