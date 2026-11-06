// Package rack is Ruby's Rack, require "rack", written in cameo:
// rack.cameo, which this package embeds and lib.Sources registers. It is
// the interface io/httpd serves and frankie is built on: Rack::Request,
// Response, Builder with middleware, URLMap, MockRequest and Utils.
package rack

import _ "embed"

// Source is the library, which require runs at the top level.
//
//go:embed rack.cameo
var Source string
