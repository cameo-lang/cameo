// Package frankie is cameo's Sinatra-like web framework, require
// "frankie", written in cameo: frankie.cameo, which this package embeds
// and lib.Sources registers. An app is a value, Frankie.app, and a Rack
// app; each verb returns a new app with the route; serve runs the app on
// the Rack server the script required, through Rack::Handler: frankie does
// no I/O of its own.
package frankie

import _ "embed"

// Source is the library, which require runs at the top level.
//
//go:embed frankie.cameo
var Source string
