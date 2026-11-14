// Package trusted holds the key to what cameo's own code alone may do.
// It is internal, so only packages of this module under src can name
// Key: a function that takes one cannot be called from a host's code.
package trusted

// Key is given to a function that trusts its caller, such as
// object.TrustedString, which makes a string without reading it.
type Key struct{}
