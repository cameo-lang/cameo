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

run_tests
