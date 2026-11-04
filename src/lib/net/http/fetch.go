package http

import (
	"encoding/base64"

	"github.com/cameo-lang/cameo/src/lib/uri"
	"github.com/cameo-lang/cameo/src/object"
)

// Fetched is what Fetch returns: the response's status, its header fields
// by lower-case name in the order they came, and its body.
type Fetched struct {
	Code    int
	Message string
	Names   []string
	Values  map[string]string
	Body    string
}

// Fetch is the request open-uri sends: a GET of u with extra header
// fields, basic authentication when user is set, and a read timeout in
// seconds when positive. It raises as Net::HTTP raises for what goes
// wrong on the way.
func Fetch(ctx object.Context, u *uri.Components, header map[string]string, user, pass string, readTimeout float64) (*Fetched, error) {
	c := connOf(u)
	if readTimeout > 0 {
		c.readTimeout = readTimeout
	}
	req := newRequest(requestClasses["GET"], "GET", u.RequestURI, u)
	for name, value := range header {
		req.headers.Set(name, value)
	}
	if user != "" {
		req.headers.Set("authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(user+":"+pass)))
	}
	res, err := perform(ctx, c, req)
	if err != nil {
		return nil, err
	}
	r := readResponse(res)
	f := &Fetched{Code: r.code, Message: r.message, Values: map[string]string{}}
	for _, name := range r.headers.names {
		f.Names = append(f.Names, name)
		f.Values[name] = r.headers.values[name]
	}
	if r.body.IsString() {
		f.Body = r.body.Str()
	}
	return f, nil
}
