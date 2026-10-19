#!/usr/bin/env bash
# Prepares a fresh Ubuntu 24.04 server to run cameo's benchmarks: Go at the
# version go.mod names, and CRuby 3.4 and 4.0 built from source into
# /opt/ruby-<version>. It is a shell script, not Ruby, because it is what
# installs Ruby. Run it as root:
#
#   ssh root@HOST 'bash -s' < benchmarks/hetzner/provision.sh
#
# GO_VERSION, RUBY34 and RUBY40 override the versions.
set -euo pipefail

GO_VERSION=${GO_VERSION:-1.26.4}
RUBY34=${RUBY34:-3.4.11}
RUBY40=${RUBY40:-4.0.7}
JOBS=$(nproc)

export DEBIAN_FRONTEND=noninteractive
apt-get -qq update
apt-get -qq install -y build-essential git curl time autoconf bison \
  libssl-dev libyaml-dev zlib1g-dev libffi-dev libreadline-dev libgmp-dev >/dev/null

# Go. benchmarks/rbgo names a newer one, which GOTOOLCHAIN=auto fetches.
arch=$(dpkg --print-architecture)
curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${arch}.tar.gz" | tar -C /usr/local -xz
ln -sf /usr/local/go/bin/go /usr/local/bin/go

build_ruby() {
  local version=$1 series=${1%.*}
  local src=/tmp/ruby-$version
  curl -fsSL "https://cache.ruby-lang.org/pub/ruby/$series/ruby-$version.tar.gz" | tar -C /tmp -xz
  (cd "$src" && ./configure --prefix="/opt/ruby-$version" --disable-install-doc >/dev/null &&
    make -j"$JOBS" >/dev/null && make install >/dev/null)
  rm -rf "$src"
}
build_ruby "$RUBY34"
build_ruby "$RUBY40"

# Ruby 3.4 is the ruby on PATH, which benchmarks/compare measures as mri.
ln -sf "/opt/ruby-$RUBY34/bin/ruby" /usr/local/bin/ruby

echo "go:   $(go version)"
for v in "$RUBY34" "$RUBY40"; do
  echo "ruby: $("/opt/ruby-$v/bin/ruby" -v)"
done
