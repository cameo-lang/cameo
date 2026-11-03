package tests

import (
	"runtime"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

func TestRbConfigDescribesCameo(t *testing.T) {
	out := runScript(t, `require "rbconfig"
c = RbConfig::CONFIG
p c["ruby_version"], c["MAJOR"], c["MINOR"], c["TEENY"], c["RUBY_PROGRAM_VERSION"] == RUBY_VERSION, c["ruby_install_name"], c["RUBY_INSTALL_NAME"], c["RUBY_ENGINE"], c["RUBY_ENGINE_VERSION"] == RUBY_ENGINE_VERSION, c["arch"] == RUBY_PLATFORM, c["host_os"], c["host_cpu"], c["host"], c["EXEEXT"], c["DLEXT"], c["PATH_SEPARATOR"]
p c["bindir"].empty?, c["prefix"] == RbConfig::TOPDIR, RbConfig.ruby.start_with?(c["bindir"]), c["libdir"] == c["prefix"] + "/lib", c.size > 30, RbConfig::MAKEFILE_CONFIG["MAJOR"], RbConfig::DESTDIR
p RbConfig.expand("$(bindir)/x") == c["bindir"] + "/x", RbConfig.expand("${MAJOR}.${MINOR}"), RbConfig.expand("$(a)/$(b)", {"a" => "1", "b" => "$(c)", "c" => "2"}), RbConfig.expand("$(nope)x"), RbConfig.expand("plain")
`)
	hostOS, cpu, vendor, exe, dlext := runtime.GOOS, runtime.GOARCH, "pc", "", "so"
	switch runtime.GOOS {
	case "darwin":
		vendor, dlext = "apple", "bundle"
	case "windows":
		hostOS, exe, dlext = "mingw32", ".exe", "dll"
	}
	switch runtime.GOARCH {
	case "amd64":
		cpu = "x86_64"
	case "arm64":
		if runtime.GOOS != "darwin" {
			cpu = "aarch64"
		}
	}
	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
	}
	assert.Equals(t, out, `"3.4.0"
"3"
"4"
"0"
true
"cameo"
"cameo"
"cameo"
true
true
"`+hostOS+`"
"`+cpu+`"
"`+cpu+"-"+vendor+"-"+hostOS+`"
"`+exe+`"
"`+dlext+`"
"`+sep+`"
false
true
true
true
true
"3"
""
true
"3.4"
"1/2"
"x"
"plain"
`)
}
