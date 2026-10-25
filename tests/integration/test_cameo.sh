#!/usr/bin/env bash
# Integration tests for the cameo binary: they run the built command the way
# a user does and check what it prints and how it exits.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/testsh.inc"

CAMEO_BIN="${CAMEO_BIN:?CAMEO_BIN names the cameo binary to test; run make integration-tests}"

test_version_prints_the_version() {
  local out
  out=$("$CAMEO_BIN" version)
  assert_match '^[0-9]+\.[0-9]+\.[0-9]+' "$out"
}

test_version_flag_matches_the_command() {
  local flag cmd
  flag=$("$CAMEO_BIN" --version)
  cmd=$("$CAMEO_BIN" version)
  assert_contains "$flag" "$cmd"
}

test_unknown_command_fails() {
  local rc=0
  "$CAMEO_BIN" no-such-command >/dev/null 2>&1 || rc=$?
  assert_neq 0 "$rc" "an unknown command exits 0"
}

test_cgi_escapes_and_indents_html() {
  local script out
  script="$(mktemp -d)/cgi.cameo"
  cat >"$script" <<'CAMEO'
require "cgi"
puts CGI.escapeURIComponent("a b+é"), CGI.unescapeHTML("&lt;&#x41;&#66;&gt;")
puts CGI.escapeElement("<br><a href='x'>", "a"), CGI.rfc1123_date(Time.at(0))
print CGI.pretty("<p><b>x</b></p>")
CAMEO
  out=$("$CAMEO_BIN" "$script")
  assert_eq $'a%20b%2B%C3%A9\n<AB>\n<br>&lt;a href=&#39;x&#39;&gt;\nThu, 01 Jan 1970 00:00:00 GMT\n<p>\n  <b>\n    x\n  </b>\n</p>' "$out"
}

test_json_round_trips_and_reports_parse_errors() {
  local script out
  script="$(mktemp -d)/json.cameo"
  cat >"$script" <<'CAMEO'
require "json"
doc = JSON.parse('{"a": [1, 2.5, null], "b": {"c": "é"}}', symbolize_names: true)
puts JSON.generate(doc), JSON.pretty_generate(doc[:b])
JSON.parse("[1,]")
CAMEO
  local rc=0
  out=$("$CAMEO_BIN" "$script" 2>&1) || rc=$?
  assert_eq 1 "$rc" "a ParserError does not fail the run"
  assert_eq $'{"a":[1,2.5,null],"b":{"c":"é"}}\n{\n  "c": "é"\n}\n'"$script"$':4: unexpected token at \']\' (JSON::ParserError)' "$out"
}

test_exit_ends_the_run_with_its_status() {
  local dir out rc
  dir="$(mktemp -d)"
  printf 'begin\n  puts "before"\n  exit 3\nensure\n  puts "ensure"\nend\nputs "after"\n' >"$dir/exit.cameo"
  rc=0
  out=$("$CAMEO_BIN" "$dir/exit.cameo" 2>&1) || rc=$?
  assert_eq 3 "$rc" "exit 3 sets the status"
  assert_eq $'before\nensure' "$out"
  printf 'abort "gone"\n' >"$dir/abort.cameo"
  rc=0
  out=$("$CAMEO_BIN" "$dir/abort.cameo" 2>&1) || rc=$?
  assert_eq 1 "$rc" "abort exits 1"
  assert_eq "gone" "$out"
}

test_benchmark_reports_the_time_inside_the_script() {
  local script out
  script="$(mktemp -d)/bench.cameo"
  cat >"$script" <<'CAMEO'
require "benchmark"
Benchmark.bm(4) { |x| x.report("sum") { (1..1000).sum } }
CAMEO
  out=$("$CAMEO_BIN" "$script" 2>&1)
  assert_match $'^ +user +system +total +real\nsum   +[0-9]+\\.[0-9]{6} +[0-9]+\\.[0-9]{6} +[0-9]+\\.[0-9]{6} \\( +[0-9]+\\.[0-9]{6}\\)$' "$out"
}

# pausing_script writes a script that busies itself for a while in one
# top-level statement, so an interrupt sent meanwhile pauses it before the
# statement after. Job control is on in the subshell that starts it: a
# background job without it ignores SIGINT.
pausing_script() {
  local script
  script="$(mktemp -d)/wait.cameo"
  cat >"$script" <<'CAMEO'
class Point
  int :x, :y
end
p = Point.new(x: 1, y: 2)
puts "before"
t = Time.now
n = 0
n += 1 while Time.now - t < 2
puts "after #{p.x + p.y}"
CAMEO
  echo "$script"
}

# interrupt_after runs the command in the background, sends SIGINT after
# the given seconds, prints its output and returns its exit status.
interrupt_after() {
  local seconds="$1"; shift
  set -m
  "$@" 2>&1 &
  local pid=$!
  set +m
  sleep "$seconds"
  kill -INT "$pid"
  wait "$pid"
}

test_interrupt_pauses_the_script_and_resume_continues_it() {
  local script out rc=0 dump
  script="$(pausing_script)"
  out=$(interrupt_after 0.5 "$CAMEO_BIN" "$script") || rc=$?
  assert_eq 3 "$rc" "a paused script exits 3; output: $out"
  assert_contains "$out" $'before
cameo: pausing before the next top-level statement'
  assert_contains "$out" "cameo: paused before line 9 of $script"
  dump=$(sed -n 's/^cameo: resume with: cameo resume //p' <<<"$out")
  assert_match '^/.*/cameo-wait-[^/]*\.dump$' "$dump"
  out=$("$CAMEO_BIN" inspect "$dump")
  assert_contains "$out" $'paused: before line 9
variables:
  p = Point(x: 1, y: 2)
'
  assert_contains "$out" $'constants: none
promises: none'
  out=$("$CAMEO_BIN" resume "$dump" 2>&1)
  assert_eq "after 3" "$out"
}

test_no_pause_lets_the_interrupt_end_the_script() {
  local script out rc=0
  script="$(pausing_script)"
  out=$(interrupt_after 0.5 "$CAMEO_BIN" --no-pause "$script") || rc=$?
  assert_eq 130 "$rc" "a script ended by SIGINT exits 130; output: $out"
  assert_eq "before" "$out"
}

test_irb_evaluates_lines_at_the_top_level() {
  local out
  out=$(printf 'x = 20\ndef twice(n)\n  n * 2\nend\ntwice(x) + 2\nexit\n' | "$CAMEO_BIN" irb)
  assert_contains "$out" "cameo(main):001> => 20"
  assert_contains "$out" "cameo(main):002* => :twice"
  assert_contains "$out" "cameo(main):003> => 42"
}

test_debug_stops_at_a_breakpoint_and_shows_the_frame() {
  local script out
  script="$(mktemp -d)/fib.cameo"
  cat >"$script" <<'CAMEO'
def fib(n)
  return n if n < 2
  fib(n - 1) + fib(n - 2)
end
puts fib(3)
CAMEO
  out=$(printf 'b 3\nc\nbt\np n * 10\nc\n' | "$CAMEO_BIN" debug "$script")
  assert_contains "$out" $'=>#0\tfib(n=3) at '"$script"':3'
  assert_contains "$out" $'  #1\t<main> at '"$script"':5'
  assert_contains "$out" "=> 30"
  # The console ends at the second stop, and the run goes on to print 2.
  assert_contains "$out" $'(rdbg) \n2'
}

test_run_builds_and_runs_the_compiled_script() {
  local script out rc=0
  script="$(mktemp -d)/fact.rb"
  cat >"$script" <<'CAMEO'
def fact(n)
  n < 2 ? 1 : n * fact(n - 1)
end
puts fact(25)
raise "done"
CAMEO
  out=$(cd "$(dirname "$script")" && "$CAMEO_BIN" run fact.rb 2>&1) || rc=$?
  assert_eq 1 "$rc" "an uncaught exception does not fail cameo run"
  assert_eq $'15511210043330985984000000\nfact.rb:5: done (RuntimeError)' "$out"
}

# A call of one of Ruby's mutators raises: cameo deviates from Ruby and
# never changes or rebinds the receiver. --ignore-mutation-errors makes the
# call return Ruby's result on a copy, the receiver left as it was.
test_mutators_raise_unless_ignored() {
  local script out rc=0
  script="$(mktemp -d)/mutate.cameo"
  cat >"$script" <<'CAMEO'
a = [1, 2]
p a.pop
p a
CAMEO
  out=$("$CAMEO_BIN" "$script" 2>&1) || rc=$?
  assert_neq 0 "$rc" "a mutator ran without an error"
  assert_contains "$out" "cameo does not support mutating calls without rebinding; use last and take (CameoError)"
  out=$("$CAMEO_BIN" --ignore-mutation-errors "$script")
  assert_eq $'2\n[1, 2]' "$out"
}

test_bang_methods_are_refused_with_the_flag_too() {
  local script out rc=0
  script="$(mktemp -d)/bang.cameo"
  printf 'x = [nil, 1]\nx.compact!\n' >"$script"
  out=$("$CAMEO_BIN" --ignore-mutation-errors "$script" 2>&1) || rc=$?
  assert_neq 0 "$rc" "a bang method ran"
  assert_contains "$out" "This is not supported, use x = x.compact instead"
}

run_tests
