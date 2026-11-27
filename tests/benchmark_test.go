package tests

import (
	"regexp"
	"strings"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The scripts are plain Ruby, and the expected outputs are Ruby 3.4's,
// with benchmark 0.4.0.

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

func TestBenchmarkJobListsItsReportsAsRubys(t *testing.T) {
	out := runScript(t, `require "benchmark"
Benchmark.bmbm do |x|
  x.report("a") { 1 }
  x.item("bb") { 2 }
  p x.list.map { |l, b| [l, b.call] }
end`)
	assert.Equals(t, strings.SplitN(out, "\n", 2)[0], `[["a", 1], ["bb", 2]]`)
}
