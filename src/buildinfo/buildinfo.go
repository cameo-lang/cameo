// Package buildinfo describes the cameo binary: its version, the commit it
// was built from, and the platform it runs on. The cameo package returns the
// version to a host, and the standard library defines Ruby's RUBY_*
// constants from all of it.
//
// The version is set at build time with
//
//	go build -ldflags "-X github.com/cameo-lang/cameo/src/buildinfo.version=0.2.3"
//
// and is otherwise the module version Go records for `go install
// ...@v0.2.3`. The revision and its date come from the version control
// information Go records when it builds inside a git checkout; `go test`,
// `go run` and a module download record none.
package buildinfo

import (
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

// version is set at build time with -ldflags "-X ...buildinfo.version=...".
var version = ""

// devVersion is the version of a binary built without a version.
const devVersion = "0.0.0-dev"

// Unknown is the revision and the date of a binary built without version
// control information.
const Unknown = "unknown"

// RubyVersion is the version of Ruby whose language and core library cameo
// follows: Ruby 3.4 as released, with the json 2.9.1 and benchmark 0.4.0
// it ships. cameo takes none of CRuby's patch releases, so the patch number
// is 0.
const RubyVersion = "3.4.0"

// Info is what a binary knows about its own build.
type Info struct {
	Version  string // cameo's version, without a leading "v"
	Revision string // the full commit hash, or Unknown
	Date     string // the commit date in UTC as YYYY-MM-DD, or Unknown
	Platform string // the platform in Ruby's spelling, as x86_64-linux
}

// Read returns the build information of the running binary.
func Read() Info {
	bi, _ := debug.ReadBuildInfo()
	return fromBuildInfo(bi, version, runtime.GOOS, runtime.GOARCH)
}

func fromBuildInfo(bi *debug.BuildInfo, ldVersion, goos, goarch string) Info {
	info := Info{Version: ldVersion, Revision: Unknown, Date: Unknown, Platform: Platform(goos, goarch)}
	if bi == nil {
		if info.Version == "" {
			info.Version = devVersion
		}
		return info
	}
	if info.Version == "" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		info.Version = strings.TrimPrefix(bi.Main.Version, "v")
	}
	if info.Version == "" {
		info.Version = devVersion
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if s.Value != "" {
				info.Revision = s.Value
			}
		case "vcs.time":
			if t, err := time.Parse(time.RFC3339, s.Value); err == nil {
				info.Date = t.UTC().Format(time.DateOnly)
			}
		}
	}
	return info
}

// Description returns the build in the form of CRuby's RUBY_DESCRIPTION,
// as "cameo 0.2.3 (2026-10-20 revision 0123456789) +ruby 3.4.0
// [x86_64-linux]". The parenthesised part names what is known of the commit
// and is left out when nothing is.
func (i Info) Description() string {
	var commit []string
	if i.Date != Unknown {
		commit = append(commit, i.Date)
	}
	if i.Revision != Unknown {
		commit = append(commit, "revision "+i.Revision[:min(10, len(i.Revision))])
	}
	s := "cameo " + i.Version
	if len(commit) > 0 {
		s += " (" + strings.Join(commit, " ") + ")"
	}
	return s + " +ruby " + RubyVersion + " [" + i.Platform + "]"
}

// Platform returns the platform that GOOS and GOARCH name, in the spelling of
// CRuby's RUBY_PLATFORM, without the OS version CRuby appends on macOS and
// FreeBSD: x86_64-linux, aarch64-linux, arm64-darwin, x64-mingw-ucrt.
func Platform(goos, goarch string) string {
	if goos == "windows" {
		switch goarch {
		case "amd64":
			return "x64-mingw-ucrt"
		case "arm64":
			return "aarch64-mingw-ucrt"
		case "386":
			return "i386-mingw32"
		}
		return goarch + "-mingw32"
	}
	cpu := goarch
	switch goarch {
	case "amd64":
		// FreeBSD calls the architecture amd64, and CRuby keeps its name.
		if goos != "freebsd" {
			cpu = "x86_64"
		}
	case "arm64":
		// Linux calls it aarch64, Apple's and the BSDs' systems arm64.
		if goos == "linux" || goos == "android" {
			cpu = "aarch64"
		}
	case "386":
		cpu = "i686"
	case "ppc64le":
		cpu = "powerpc64le"
	case "ppc64":
		cpu = "powerpc64"
	case "loong64":
		cpu = "loongarch64"
	}
	return cpu + "-" + goos
}
