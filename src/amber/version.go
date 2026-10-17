package amber

// version is set at build time with -ldflags "-X ...amber.version=...".
var version = "0.0.0-dev"

// Version returns the version of amber this binary was built as.
func Version() string { return version }
