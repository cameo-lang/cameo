// Package tsort is Ruby's tsort library, written in cameo: tsort.cameo
// defines TSort, which require runs.
package tsort

import _ "embed"

// Source is the library.
//
//go:embed tsort.cameo
var Source string
