package tests

import (
	"regexp"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The scripts are plain Ruby, and the expected outputs are Ruby 3.4's,
// with benchmark 0.4.0.

func TestBenchmarkTmsComputesAndFormatsAsRubys(t *testing.T) {
	out := runScript(t, `require "benchmark"
t = Benchmark::Tms.new(1.5, 0.25, 0.0, 0.0, 2.0, "x")
u = Benchmark::Tms.new(0.5, 0.75, 0.0, 0.0, 1.0)
p t.utime, t.stime, t.cutime, t.cstime, t.real, t.total, t.label
p t.to_a, t.to_h
print t.to_s, t
s = t + u
p s.to_a, (t - u).to_a, (t * 2).to_a, (t / 4).to_a
p Benchmark::Tms.new.to_a, Benchmark::Tms.new(1, 2).total
puts t.format("%n: %u user, %y sys, %t total, %r real")
puts t.format("%-5n|%.2u|%3.1r| %s", "extra")
puts Benchmark::CAPTION.inspect, Benchmark::FORMAT.inspect, Benchmark::VERSION
r = Benchmark.realtime { 1 + 1 }
p r.class, r >= 0
m = Benchmark.measure("lbl") { 1 }
p m.class, m.label, m.real >= 0
p t.add { 1 }.class
`)
	assert.Equals(t, out, `1.5
0.25
0.0
0.0
2.0
1.75
"x"
["x", 1.5, 0.25, 0.0, 0.0, 2.0]
{label: "x", utime: 1.5, stime: 0.25, cutime: 0.0, cstime: 0.0, real: 2.0}
  1.500000   0.250000   1.750000 (  2.000000)
  1.500000   0.250000   1.750000 (  2.000000)
["", 2.0, 1.0, 0.0, 0.0, 3.0]
["", 1.0, -0.5, 0.0, 0.0, 1.0]
["", 3.0, 0.5, 0.0, 0.0, 4.0]
["", 0.375, 0.0625, 0.0, 0.0, 0.5]
["", 0.0, 0.0, 0.0, 0.0, 0.0]
3.0
x: 1.500000 user, 0.250000 sys, 1.750000 total, (2.000000) real
x    |1.50|(2.0)| extra
"      user     system      total        real\n"
"%10.6u %10.6y %10.6t %10.6r\n"
0.4.0
Float
true
Benchmark::Tms
"lbl"
true
Benchmark::Tms
`)
}

// times are a time Benchmark measured, which differs from run to run.
var times = regexp.MustCompile(`-?[0-9]+\.[0-9]{6}`)

func TestBenchmarkReportsLayOutAsRubys(t *testing.T) {
	out := runScript(t, `require "benchmark"
n = 2000
r = Benchmark.bm(7) do |x|
  x.report("for:")   { for i in 1..n; a = "1"; end }
  x.report("times:") { n.times do   ; a = "1"; end }
  x.report("upto:")  { 1.upto(n) do ; a = "1"; end }
end
p r.size, r.map(&:label), r.first.class
Benchmark.bm do |x|
  x.report { 1 }
  x.report("a long label") { 1 }
end
Benchmark.benchmark(Benchmark::CAPTION, 7, Benchmark::FORMAT, ">total:", ">avg:") do |x|
  tf = x.report("for:")   { 1 }
  tt = x.report("times:") { 2 }
  [tf + tt, (tf + tt) / 2]
end
b = Benchmark.bmbm do |x|
  x.report("sort!") { [3, 1, 2].sort }
  x.report("sort")  { [3, 1, 2].sort }
end
p b.map(&:label)
`)
	assert.Equals(t, times.ReplaceAllString(out, "T"), `              user     system      total        real
for:      T   T   T (  T)
times:    T   T   T (  T)
upto:     T   T   T (  T)
3
["for:", "times:", "upto:"]
Benchmark::Tms
                  user     system      total        real
              T   T   T (  T)
a long label  T   T   T (  T)
              user     system      total        real
for:      T   T   T (  T)
times:    T   T   T (  T)
>total:   T   T   T (  T)
>avg:     T   T   T (  T)
Rehearsal -----------------------------------------
sort!   T   T   T (  T)
sort    T   T   T (  T)
-------------------------------- total: Tsec

            user     system      total        real
sort!   T   T   T (  T)
sort    T   T   T (  T)
["sort!", "sort"]
`)
}

func TestBenchmarkWithoutABlockRaisesAsRubys(t *testing.T) {
	out := runScript(t, `require "benchmark"
[-> { Benchmark.measure }, -> { Benchmark.realtime }, -> { Benchmark.bm }, -> { Benchmark.bmbm { |x| x.report("a") } }, -> { Benchmark::Tms.new.add }].each do |f|
  f.call
rescue => e
  puts "#{e.class}: #{e.message}"
end
p require("benchmark")
`)
	assert.Equals(t, out, "LocalJumpError: no block given (yield)\n"+
		"LocalJumpError: no block given (yield)\n"+
		"LocalJumpError: no block given (yield)\n"+
		"ArgumentError: no block\n"+
		"LocalJumpError: no block given (yield)\n"+
		"false\n")
}

func TestBenchmarkTmsShowsItsTimes(t *testing.T) {
	// Ruby shows an address before the instance variables; a cameo value
	// has none.
	out := runScript(t, `
p Benchmark::Tms.new(1.5, 0.25, 0.0, 0.0, 2.0, "x")
class Timer
  include Benchmark
  def run = measure("in a class") { 1 }.label
end
p Timer.new.run`)
	assert.Equals(t, out, "#<Benchmark::Tms @utime=1.5, @stime=0.25, @cutime=0.0, @cstime=0.0, @real=2.0, @label=\"x\", @total=1.75>\n\"in a class\"\n")
}

func TestBenchmarkMeasuresTheWorkOfAsyncCalls(t *testing.T) {
	// The CPU times are the process's, so they include what workers ran
	// while the block waited for them.
	out := runScript(t, `
include Cameo
class Spin
  def go(n) = (1..n).sum { |i| i * i }
end
t = Benchmark.measure { await((1..4).map { async(Spin.new, :go, 300_000) }) }
p t.real > 0, t.utime + t.stime > 0`)
	assert.Equals(t, out, "true\ntrue\n")
}

func TestBenchmarkJobListsItsReportsAsRubys(t *testing.T) {
	out := runScript(t, `require "benchmark"
Benchmark.bmbm do |x|
  x.report("a") { 1 }
  x.item("bb") { 2 }
  p x.list.map { |l, b| [l, b.call] }
end`)
	assert.Equals(t, strings.SplitN(out, "\n", 2)[0], `[["a", 1], ["bb", 2]]`)
}
