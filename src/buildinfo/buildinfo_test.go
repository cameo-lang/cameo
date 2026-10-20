package buildinfo

import (
	"runtime/debug"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

func TestPlatformUsesRubysSpelling(t *testing.T) {
	assert.Equals(t, Platform("linux", "amd64"), "x86_64-linux")
	assert.Equals(t, Platform("linux", "arm64"), "aarch64-linux")
	assert.Equals(t, Platform("darwin", "arm64"), "arm64-darwin")
	assert.Equals(t, Platform("darwin", "amd64"), "x86_64-darwin")
	assert.Equals(t, Platform("windows", "amd64"), "x64-mingw-ucrt")
	assert.Equals(t, Platform("windows", "arm64"), "aarch64-mingw-ucrt")
	assert.Equals(t, Platform("freebsd", "amd64"), "amd64-freebsd")
	assert.Equals(t, Platform("linux", "386"), "i686-linux")
	assert.Equals(t, Platform("linux", "ppc64le"), "powerpc64le-linux")
	assert.Equals(t, Platform("linux", "riscv64"), "riscv64-linux")
}

func TestReadTakesRevisionAndDateFromVCS(t *testing.T) {
	bi := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "e51014f9c05aa65cbf203442d37fef7c12390015"},
			{Key: "vcs.time", Value: "2026-10-20T23:30:00+02:00"},
		},
	}
	info := fromBuildInfo(bi, "0.2.3", "linux", "amd64")
	assert.Equals(t, info, Info{
		Version:  "0.2.3",
		Revision: "e51014f9c05aa65cbf203442d37fef7c12390015",
		Date:     "2026-10-20",
		Platform: "x86_64-linux",
	})
	assert.Equals(t, info.Description(), "cameo 0.2.3 (2026-10-20 revision e51014f9c0) +ruby 3.4.0 [x86_64-linux]")
}

func TestReadTakesVersionFromModule(t *testing.T) {
	bi := &debug.BuildInfo{Main: debug.Module{Version: "v0.2.2"}}
	info := fromBuildInfo(bi, "", "darwin", "arm64")
	assert.Equals(t, info, Info{Version: "0.2.2", Revision: Unknown, Date: Unknown, Platform: "arm64-darwin"})
	assert.Equals(t, info.Description(), "cameo 0.2.2 +ruby 3.4.0 [arm64-darwin]")
}

func TestReadFallsBackWithoutBuildInformation(t *testing.T) {
	info := fromBuildInfo(&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "", "linux", "arm64")
	assert.Equals(t, info, Info{Version: "0.0.0-dev", Revision: Unknown, Date: Unknown, Platform: "aarch64-linux"})
	assert.Equals(t, fromBuildInfo(nil, "", "linux", "arm64"), info)
}

func TestLinkerVersionWinsOverModuleVersion(t *testing.T) {
	bi := &debug.BuildInfo{Main: debug.Module{Version: "v0.2.2"}}
	assert.Equals(t, fromBuildInfo(bi, "0.3.0-rc1", "linux", "amd64").Version, "0.3.0-rc1")
}
