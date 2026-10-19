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

run_tests
