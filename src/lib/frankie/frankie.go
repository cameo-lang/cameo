// Package frankie is cameo's Sinatra-like web framework, require
// "frankie", written in cameo: frankie.cameo, which this package embeds
// and lib.Sources registers. An app is a value, Frankie.app, and a Rack
// app; each verb returns a new app with the route; serve runs the app on
// io/httpd, the one I/O it does, and only in a program that allows it.
package frankie

import _ "embed"

// Source is the library, which require runs at the top level.
//
//go:embed frankie.cameo
var Source string
