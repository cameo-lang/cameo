package cameo

import "github.com/cameo-lang/cameo/src/buildinfo"

// Version returns the version of cameo this binary was built as. The
// buildinfo package says where it comes from.
func Version() string { return buildinfo.Read().Version }
