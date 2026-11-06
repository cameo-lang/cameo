#!/usr/bin/env bash
# Runs every example against a cameo binary and compares what it prints on
# standard output with the example's .expected file: tinygo/examples.sh BIN.
# The Go tests run the examples in process; this holds another build of the
# binary, TinyGo's, to the same output.
set -uo pipefail

bin=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
cd "$(dirname "$0")/../examples"
pass=0
failed=()
for script in *.cameo async/*.rb; do
  expected="${script%.*}.expected"
  [ -f "$expected" ] || continue
  if diff -q <(cd "$(dirname "$script")" && "$bin" "$(basename "$script")" 2>/dev/null) "$expected" >/dev/null; then
    pass=$((pass + 1))
  else
    failed+=("$script")
  fi
done
echo "examples: ${pass} passed, ${#failed[@]} failed"
for f in "${failed[@]}"; do echo "  FAIL: $f"; done
[ ${#failed[@]} -eq 0 ]
