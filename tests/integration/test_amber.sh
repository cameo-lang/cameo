#!/usr/bin/env bash
# Integration tests for the amber binary: they run the built command the way
# a user does and check what it prints and how it exits.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/testsh.inc"

AMBER_BIN="${AMBER_BIN:?AMBER_BIN names the amber binary to test; run make integration-tests}"

test_version_prints_the_version() {
  local out
  out=$("$AMBER_BIN" version)
  assert_match '^[0-9]+\.[0-9]+\.[0-9]+' "$out"
}

test_version_flag_matches_the_command() {
  local flag cmd
  flag=$("$AMBER_BIN" --version)
  cmd=$("$AMBER_BIN" version)
  assert_contains "$flag" "$cmd"
}

test_unknown_command_fails() {
  local rc=0
  "$AMBER_BIN" no-such-command >/dev/null 2>&1 || rc=$?
  assert_neq 0 "$rc" "an unknown command exits 0"
}

run_tests
