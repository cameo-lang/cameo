package cameo

// version is set at build time with -ldflags "-X ...cameo.version=...".
var version = "0.0.0-dev"

// Version returns the version of cameo this binary was built as.
func Version() string { return version }
