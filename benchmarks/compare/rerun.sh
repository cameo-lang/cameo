#!/usr/bin/env bash
# Reruns the whole comparison: every case of benchmarks/ruby/works on MRI,
# cameo, warmruby and rbgo, into benchmarks/compare/results.tsv. It runs on
# Linux and macOS and needs Go and a Ruby of 3.1 or later on PATH, which is
# also the Ruby measured as mri; the first run downloads warmruby's module.
#
#   benchmarks/compare/rerun.sh [compare.rb's options]
#
# JOBS sets how many cases run at once, half the CPUs by default: more
# finishes sooner but lets the runs disturb each other's times. Options go
# to compare.rb, so --min-time 0.5 or --only cameo work here too.
set -euo pipefail

cd "$(dirname "$0")/../.."

for tool in go ruby; do
  command -v "$tool" >/dev/null || { echo "rerun.sh: $tool is not on PATH" >&2; exit 1; }
done
if ! ruby -e 'exit(Gem::Version.new(RUBY_VERSION) >= Gem::Version.new("3.1"))'; then
  echo "rerun.sh: compare.rb needs Ruby 3.1 or later, and $(command -v ruby) is $(ruby -e 'print RUBY_VERSION')" >&2
  echo "rerun.sh: on macOS, brew install ruby and put it first on PATH" >&2
  exit 1
fi

cpus=$(getconf _NPROCESSORS_ONLN 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo 4)
jobs=${JOBS:-$(( cpus / 2 > 0 ? cpus / 2 : 1 ))}

echo "machine: $(uname -sm), $cpus CPUs, $jobs cases at once"
echo "mri:     $(ruby -v)"
echo "go:      $(go version)"
echo

exec ruby benchmarks/compare/compare.rb --jobs "$jobs" "$@"
