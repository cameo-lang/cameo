#!/usr/bin/env bash
# Times the scripts here on the interpreter, compiled with cameo build, and
# on Ruby, and prints a TSV row per case: the best of three processes'
# in-script seconds, and the result. Runs from any directory, on Linux and
# macOS, with bash 3.2.
#
#   benchmarks/aot/run.sh
#
# RUBIES lists the ruby commands to time (default: ruby); one that has
# YJIT is timed with it too.
set -euo pipefail
cd "$(dirname "$0")/../.."

go build -o bin/cameo ./src/cmd/cameo
{
  echo "machine: $(uname -sm), $(sysctl -n machdep.cpu.brand_string 2>/dev/null || sed -n 's/^model name[[:space:]]*: //p' /proc/cpuinfo | head -n 1)"
  echo "go: $(go env GOVERSION), cameo: $(git rev-parse --short HEAD)"
} >&2

# best runs a command three times and prints the best seconds and the
# result. The command prints "result seconds" as its last line.
best() {
  local secs="" res="" out r t
  for _ in 1 2 3; do
    if ! out=$("$@"); then
      echo "run.sh: failed: $*" >&2
      return 1
    fi
    read -r r t <<<"$(tail -n 1 <<<"$out")"
    res=$r
    if [[ -z $secs ]] || awk "BEGIN{exit !($t < $secs)}"; then secs=$t; fi
  done
  printf "%.3f\t%s" "$secs" "$res"
}

printf "who\tscript\tsecs\tresult\n"
for script in benchmarks/aot/*.rb; do
  name=$(basename "$script" .rb)
  bin/cameo build "$script" "bin/aot/$name" 2>&1 >/dev/null | grep -v '^bin/' | sed "s/^/$name: /" >&2
  go build -o "bin/aot/$name/$name" "./bin/aot/$name"
  printf "interpreted\t%s\t%s\n" "$name" "$(best bin/cameo "$script")"
  printf "compiled\t%s\t%s\n" "$name" "$(best "bin/aot/$name/$name")"
  for ruby in ${RUBIES:-ruby}; do
    command -v "$ruby" >/dev/null || continue
    v=$("$ruby" -e 'print RUBY_VERSION')
    printf "ruby-%s\t%s\t%s\n" "$v" "$name" "$(best "$ruby" "$script")"
    if "$ruby" --yjit -e 'exit(defined?(RubyVM::YJIT) && RubyVM::YJIT.enabled? ? 0 : 1)' 2>/dev/null; then
      printf "ruby-%s-yjit\t%s\t%s\n" "$v" "$name" "$(best "$ruby" --yjit "$script")"
    fi
  done
done
