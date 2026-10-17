// Package amber is the host API of amber, a small language with Ruby's
// syntax whose values are immutable. It is the package a Go program imports
// to run amber scripts; the interpreter behind it is still to come.
package amber

// version is set at build time with -ldflags "-X ...amber.version=...".
var version = "0.0.0-dev"

// Version returns the version of amber this binary was built as.
func Version() string { return version }
