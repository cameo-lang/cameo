#!/usr/bin/env bash
# Runs a TinyGo command on cameo: tinygo/build.sh build|run [flags] PACKAGE,
# as the Makefile's tinygo targets call it.
#
# protobuf-go calls reflect's MethodByName when it first uses a generated
# message, which its protodesc package does as the program starts, and
# TinyGo's reflect panics there. So the build copies the protobuf-go that
# go.mod names out of the module cache into bin/tinygo, applies
# tinygo/protobuf.patch, and points TinyGo at the copy through a workspace
# file there. go.mod and the Go toolchain's build are left as they are.
set -euo pipefail

root=$(cd "$(dirname "$0")/.." && pwd)
work="$root/bin/tinygo"
cd "$root"

command -v tinygo >/dev/null 2>&1 || {
  echo "tinygo is not installed. See https://tinygo.org/getting-started/install/" >&2
  exit 1
}

version=$(go list -m -f '{{.Version}}' google.golang.org/protobuf)
pristine=$(go list -m -f '{{.Dir}}' google.golang.org/protobuf)
copy="$work/protobuf@$version"
if [ ! -f "$copy/.patched" ]; then
  rm -rf "$copy"
  mkdir -p "$work"
  cp -R "$pristine" "$copy"
  chmod -R u+w "$copy"
  patch -s -p1 -d "$copy" <"$root/tinygo/protobuf.patch"
  touch "$copy/.patched"
fi

cat >"$work/go.work" <<WORK
go $(go mod edit -json | sed -n 's/^\t"Go": "\(.*\)",$/\1/p')

use $root

replace google.golang.org/protobuf => ./protobuf@$version
WORK

GOWORK="$work/go.work" exec tinygo "$@"
