// Package uri is Ruby's uri library, built into cameo: URI.parse, join,
// split, URI(), the classes URI::Generic, HTTP, HTTPS, FTP, File, WS, WSS,
// LDAP, LDAPS and MailTo with their components, and the www-form encoders.
//
// It follows uri 1.0.2, the version Ruby 3.4 ships. Go's net/url reads a
// URI, resolves a reference and escapes a component; the components land
// in Ruby's places and the refusals carry Ruby's messages. A URI is an instance of its class
// with the components as typed fields, so it is a protobuf message like
// every cameo instance, with and == work on it, and u.host = "x" rebinds u
// to a copy, checked as Ruby's setter checks the value.
package uri

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
)

// The fields of URI::Generic, in Ruby's component order without the
// registry, which the RFC 3986 parser never sets. Every subclass has them.
const (
	fScheme = iota
	fUserinfo
	fHost
	fPort
	fPath
	fOpaque
	fQuery
	fFragment
	fieldCount
)

var fieldNames = [fieldCount]string{"scheme", "userinfo", "host", "port", "path", "opaque", "query", "fragment"}

var (
	uriModule    = object.NewModule("URI")
	errorClass   = object.NewClass("Error", object.StandardError)
	invalidURI   = object.NewClass("InvalidURIError", errorClass)
	invalidComp  = object.NewClass("InvalidComponentError", errorClass)
	badURI       = object.NewClass("BadURIError", errorClass)
	genericClass = object.NewClass("Generic", object.ObjectClass)
	httpClass    = object.NewClass("HTTP", genericClass)
	httpsClass   = object.NewClass("HTTPS", httpClass)
	ftpClass     = object.NewClass("FTP", genericClass)
	fileClass    = object.NewClass("File", genericClass)
	wsClass      = object.NewClass("WS", genericClass)
	wssClass     = object.NewClass("WSS", wsClass)
	ldapClass    = object.NewClass("LDAP", genericClass)
	ldapsClass   = object.NewClass("LDAPS", ldapClass)
	mailtoClass  = object.NewClass("MailTo", genericClass)
	parserClass  = object.NewClass("RFC3986_Parser", object.ObjectClass)
	parserValue  object.Value
)

// schemes maps an upper-case scheme to its class, as URI.scheme_list does.
var schemes = map[string]*object.Class{
	"HTTP": httpClass, "HTTPS": httpsClass, "FTP": ftpClass, "FILE": fileClass,
	"WS": wsClass, "WSS": wssClass, "LDAP": ldapClass, "LDAPS": ldapsClass, "MAILTO": mailtoClass,
}

// defaultPorts are the classes' default ports; a class not here has none.
var defaultPorts = map[*object.Class]int64{
	httpClass: 80, httpsClass: 443, ftpClass: 21, wsClass: 80, wssClass: 443, ldapClass: 389, ldapsClass: 636,
}

// components are each class's components, what select takes and == and
// build go by.
var components = map[*object.Class][]string{
	genericClass: {"scheme", "userinfo", "host", "port", "registry", "path", "opaque", "query", "fragment"},
	httpClass:    {"scheme", "userinfo", "host", "port", "path", "query", "fragment"},
	ftpClass:     {"scheme", "userinfo", "host", "port", "path", "typecode"},
	fileClass:    {"scheme", "host", "path"},
	ldapClass:    {"scheme", "host", "port", "dn", "attributes", "scope", "filter", "extensions"},
	mailtoClass:  {"scheme", "to", "headers"},
}

// Install defines the URI module, its classes and Kernel#URI. lib.Install
// calls it once.
func Install() {
	m := uriModule
	object.ObjectClass.Consts["URI"] = object.ClassValue(m)
	m.Consts["VERSION"] = object.String("1.0.2")
	for _, c := range []*object.Class{errorClass, invalidURI, invalidComp, badURI, genericClass, httpClass, httpsClass, ftpClass, fileClass, wsClass, wssClass, ldapClass, ldapsClass, mailtoClass, parserClass} {
		c.Lexical = m
		m.Consts[c.Name] = object.ClassValue(c)
	}
	m.Consts["Parser"] = object.ClassValue(parserClass)
	parserValue = object.InstanceValue(&object.Instance{Cls: parserClass})
	m.Consts["DEFAULT_PARSER"] = parserValue
	m.Consts["RFC3986_PARSER"] = parserValue
	m.Consts["RFC2396_PARSER"] = parserValue
	// Generic includes URI, as Ruby's does: URI::Generic.ancestors names it.
	genericClass.Include(m)

	declareFields(genericClass)
	for _, c := range []*object.Class{httpClass, httpsClass, ftpClass, fileClass, wsClass, wssClass, ldapClass, ldapsClass, mailtoClass} {
		inheritFields(c)
	}
	for c, cs := range components {
		c.Consts["COMPONENT"] = symbols(cs)
	}
	genericClass.Consts["DEFAULT_PORT"] = object.Nil
	for c, port := range defaultPorts {
		c.Consts["DEFAULT_PORT"] = object.Int(port)
	}
	genericClass.Consts["USE_REGISTRY"] = object.False

	installModule()
	installGeneric()
	installSubclasses()
	installForms()
	installParser()
}

// declareFields gives Generic its fields: strings, and an Integer port.
func declareFields(c *object.Class) {
	taken := func(number int64) bool {
		for _, f := range c.Fields {
			if f.Number == number {
				return true
			}
		}
		return false
	}
	for i, name := range fieldNames {
		typ := object.ScalarField("string")
		if i == fPort {
			typ = object.ScalarField("int")
		}
		f := &object.Field{Name: name, Number: protogen.FieldNumber(name, taken), Type: typ, Owner: c}
		c.FieldIndex[name] = len(c.Fields)
		c.Fields = append(c.Fields, f)
	}
}

// inheritFields copies the superclass's fields into c, as a class body
// extending a class does.
func inheritFields(c *object.Class) {
	for _, f := range c.Super.Fields {
		c.FieldIndex[f.Name] = len(c.Fields)
		c.Fields = append(c.Fields, f)
	}
}

func symbols(names []string) object.Value {
	out := make([]object.Value, len(names))
	for i, n := range names {
		out[i] = object.Symbol(n)
	}
	return object.NewList(out...)
}

// parts are a URI's components as Go reads them: a nil string pointer is
// Ruby's nil.
type parts struct {
	scheme, userinfo, host, path, opaque, query, fragment *string
	port                                                  *int64
}

func str(s string) *string { return &s }

func read(v object.Value) parts {
	f := v.Instance().Fields
	get := func(i int) *string {
		if f[i].IsZero() || f[i].IsNil() {
			return nil
		}
		return str(f[i].Str())
	}
	p := parts{scheme: get(fScheme), userinfo: get(fUserinfo), host: get(fHost), path: get(fPath), opaque: get(fOpaque), query: get(fQuery), fragment: get(fFragment)}
	if n, ok := f[fPort].AsInt(); ok {
		p.port = &n
	}
	return p
}

func build(cls *object.Class, p parts) object.Value {
	fields := make([]object.Value, len(cls.Fields))
	for i := range fields {
		fields[i] = object.Nil
	}
	set := func(i int, s *string) {
		if s != nil {
			fields[i] = object.String(*s)
		}
	}
	set(fScheme, p.scheme)
	set(fUserinfo, p.userinfo)
	set(fHost, p.host)
	set(fPath, p.path)
	set(fOpaque, p.opaque)
	set(fQuery, p.query)
	set(fFragment, p.fragment)
	if p.port != nil {
		fields[fPort] = object.Int(*p.port)
	}
	return object.InstanceValue(&object.Instance{Cls: cls, Fields: fields})
}

// isURI reports whether v is a URI::Generic or a subclass instance.
func isURI(v object.Value) bool {
	inst, ok := v.AsInstance()
	return ok && inst.Cls.IsA(genericClass)
}

func defaultPort(cls *object.Class) *int64 {
	for c := cls; c != nil; c = c.Super {
		if port, ok := defaultPorts[c]; ok {
			return &port
		}
	}
	return nil
}

func componentsOf(cls *object.Class) []string {
	for c := cls; c != nil; c = c.Super {
		if cs, ok := components[c]; ok {
			return cs
		}
	}
	return components[genericClass]
}

// Ruby's RFC 3986 grammar, as the parser's regular expressions have it,
// in Go's syntax: possessive quantifiers become greedy ones, \h is spelled
// out, and the named recursions are inlined.
const (
	hex      = `[0-9A-Fa-f]`
	h16      = `[0-9A-Fa-f]{1,4}`
	decOctet = `(?:[1-9]\d|1\d{2}|2[0-4]\d|25[0-5]|\d)`
	ipv4     = decOctet + `\.` + decOctet + `\.` + decOctet + `\.` + decOctet
	ls32     = `(?:` + h16 + `:` + h16 + `|` + ipv4 + `)`
	ipv6     = `(?:(?:` + h16 + `:){6}` + ls32 +
		`|::(?:` + h16 + `:){5}` + ls32 +
		`|` + hex + `{0,4}::(?:` + h16 + `:){4}` + ls32 +
		`|(?:(?:` + h16 + `:)?` + h16 + `)?::(?:` + h16 + `:){3}` + ls32 +
		`|(?:(?:` + h16 + `:){0,2}` + h16 + `)?::(?:` + h16 + `:){2}` + ls32 +
		`|(?:(?:` + h16 + `:){0,3}` + h16 + `)?::` + h16 + `:` + ls32 +
		`|(?:(?:` + h16 + `:){0,4}` + h16 + `)?::` + ls32 +
		`|(?:(?:` + h16 + `:){0,5}` + h16 + `)?::` + h16 +
		`|(?:(?:` + h16 + `:){0,6}` + h16 + `)?::)`
	ipFuture  = `v` + hex + `+\.[!$&-.0-9:;=A-Z_a-z~]+`
	ipLiteral = `\[(?:` + ipv6 + `|` + ipFuture + `)\]`
	regName   = `(?:%` + hex + hex + `|[!$&-.0-9;=A-Z_a-z~])*`
	host      = `(?:` + ipLiteral + `|` + ipv4 + `|` + regName + `)`
	userinfo  = `(?:%` + hex + hex + `|[!$&-.0-9:;=A-Z_a-z~])*`
	scheme    = `[A-Za-z][+\-.0-9A-Za-z]*`
	// seg is a run of path characters, slashes included; segFirst one that
	// is not a slash, where Ruby writes (?!/), and segNC one without a
	// colon, the first of a relative path that is no scheme.
	seg      = `(?:%` + hex + hex + `|[!$&-.0-9:;=@-Z_a-z~/])`
	segFirst = `(?:%` + hex + hex + `|[!$&-.0-9:;=@-Z_a-z~])`
	segNC    = `(?:%` + hex + hex + `|[!$&-.0-9;=@-Z_a-z~])`
	fragment = `(?:%` + hex + hex + `|[!$&-.0-9:;=@-Z_a-z~/?])*`
	query    = `[^#]*`
)

var (
	// rfc3986URI matches an absolute URI; the groups are scheme, userinfo,
	// host, port, path-abempty, path-absolute, path-rootless, query,
	// fragment. path-empty is the absence of the other three.
	rfc3986URI = regexp.MustCompile(`^(` + scheme + `):(?://(?:(` + userinfo + `)@)?(` + host + `)(?::(\d*))?(/` + seg + `*)?|(/(?:` + segFirst + seg + `*)?)|(` + segFirst + seg + `*)|)(?:\?(` + query + `))?(?:#(` + fragment + `))?$`)
	// rfc3986Relative matches a relative reference; the groups are
	// userinfo, host, port, path-abempty, path-absolute, path-noscheme,
	// query, fragment.
	rfc3986Relative = regexp.MustCompile(`^(?://(?:(` + userinfo + `)@)?(` + host + `)?(?::(\d*))?(/` + seg + `*)?|(/` + seg + `*)|(` + segNC + `+(?:/` + seg + `*)?)|)(?:\?(` + query + `))?(?:#(` + fragment + `))?$`)

	// The component checks of the setters.
	schemeRe   = regexp.MustCompile(`^` + scheme + `$`)
	userinfoRe = regexp.MustCompile(`^` + userinfo + `$`)
	hostRe     = regexp.MustCompile(`^` + host + `$`)
	absPathRe  = regexp.MustCompile(`^/` + seg + `*$`)
	relPathRe  = regexp.MustCompile(`^` + segFirst + seg + `*$`)
	portRe     = regexp.MustCompile(`^[\t\n\f\r ]*\d*[\t\n\f\r ]*$`)
	opaqueRe   = regexp.MustCompile(`^(?:[^/].*)?$`)
	// A % that no two hex digits follow.
	badEscapeRe = regexp.MustCompile(`%(?:[^0-9A-Fa-f]|$|[0-9A-Fa-f](?:[^0-9A-Fa-f]|$))`)
)

// split is URI.split: the nine components of s as Strings, or nil, with
// the port as a String. Go's net/url reads the string; what it reads is
// put in Ruby's places: the scheme as written, the host with its
// brackets, an empty host when the authority is there and empty, the
// opaque part with its query, and the path as written. The registry is
// always nil.
func split(ctx object.Context, s string) ([]*string, error) {
	if !object.ASCIIOnly(s) {
		return nil, ctx.Raise(invalidURI, "URI must be ascii only %s", dump(s))
	}
	bad := func() error { return ctx.Raise(invalidURI, "bad URI (is not URI?): %s", object.QuoteString(s)) }
	for i := 0; i < len(s); i++ {
		if c := s[i]; c <= ' ' || c == 0x7f || strings.IndexByte("\"<>\\^`{|}", c) >= 0 {
			return nil, bad()
		}
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, bad()
	}
	out := make([]*string, 9)
	rest := s
	if u.Scheme != "" {
		out[0] = str(s[:len(u.Scheme)])
		rest = s[len(u.Scheme)+1:]
	}
	fragment := func() *string {
		if i := strings.IndexByte(s, '#'); i >= 0 {
			return str(s[i+1:])
		}
		return nil
	}
	if u.Opaque != "" {
		opaque := u.Opaque
		if u.ForceQuery || u.RawQuery != "" {
			opaque += "?" + u.RawQuery
		}
		out[6] = str(opaque)
		out[8] = fragment()
		return out, nil
	}
	if strings.HasPrefix(rest, "//") {
		if u.User != nil {
			out[1] = str(u.User.String())
		}
		host := u.Host
		if i := strings.LastIndexByte(host, ':'); i >= 0 && !strings.HasSuffix(host, "]") {
			out[3] = str(host[i+1:])
			host = host[:i]
		}
		out[2] = str(host)
	}
	out[5] = str(u.EscapedPath())
	if u.ForceQuery || u.RawQuery != "" {
		out[7] = str(u.RawQuery)
	}
	out[8] = fragment()
	return out, nil
}

// dump is Ruby's String#dump: inspect with every character past ASCII as
// a \u escape.
func dump(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(&b, `\x%02X`, s[i])
		case r >= 0x80:
			if r < 0x10000 {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				fmt.Fprintf(&b, `\u{%X}`, r)
			}
		default:
			q := object.QuoteString(string(r))
			b.WriteString(q[1 : len(q)-1])
		}
		i += size
	}
	b.WriteByte('"')
	return b.String()
}

// classFor is URI.for: the class registered for the scheme, or Generic.
func classFor(scheme *string) *object.Class {
	if scheme == nil {
		return genericClass
	}
	if c, ok := schemes[strings.ToUpper(*scheme)]; ok {
		return c
	}
	return genericClass
}

// parse is URI.parse: split, then the class's new without argument
// checks, as Ruby's does.
func parse(ctx object.Context, v object.Value) (object.Value, error) {
	s, ok := v.AsString()
	if !ok {
		shown, err := ctx.Inspect(v)
		if err != nil {
			return object.Value{}, err
		}
		return object.Value{}, ctx.Raise(invalidURI, "bad URI (is not URI?): %s", shown)
	}
	c, err := split(ctx, s)
	if err != nil {
		return object.Value{}, err
	}
	return construct(ctx, classFor(c[0]), c, false)
}

// construct is URI::Generic#initialize: the nine components, checked as
// the setters check them when argCheck is set, as new does from a
// script, and taken as the parser read them otherwise.
func construct(ctx object.Context, cls *object.Class, c []*string, argCheck bool) (object.Value, error) {
	var p parts
	set := func(i int, v *string) error {
		var err error
		switch i {
		case 0:
			if argCheck {
				err = checkScheme(ctx, v)
			}
			if v != nil {
				v = str(strings.ToLower(*v))
			}
			p.scheme = v
		case 1:
			if argCheck {
				err = checkUserinfo(ctx, p, v)
			}
			if cls.IsA(fileClass) {
				return nil
			}
			p.userinfo = v
		case 2:
			if argCheck {
				if err = checkHost(ctx, p, v); err == nil && v != nil && strings.Contains(*v, ":") && !strings.HasPrefix(*v, "[") {
					v = str("[" + *v + "]")
				}
			}
			if cls.IsA(fileClass) && (v == nil || *v == "localhost") {
				v = str("")
			}
			p.host = v
		case 3:
			if argCheck {
				err = checkPort(ctx, p, v)
			}
			if cls.IsA(fileClass) {
				return nil
			}
			if v != nil && *v != "" {
				n, convErr := strconv.ParseInt(strings.TrimSpace(*v), 10, 64)
				if convErr != nil {
					n = 0
				}
				p.port = &n
			}
		case 5:
			if argCheck {
				err = checkPath(ctx, p, v)
			}
			p.path = v
		case 6:
			if argCheck {
				err = checkOpaque(ctx, p, v)
			}
			p.opaque = v
		case 7:
			if v != nil && p.opaque != nil {
				return ctx.Raise(invalidURI, "query conflicts with opaque")
			}
			if v != nil {
				q, qErr := escapeQuery(ctx, *v)
				if qErr != nil {
					return qErr
				}
				v = str(q)
			}
			p.query = v
		case 8:
			if v != nil {
				v = str(escapeFragment(*v))
			}
			p.fragment = v
		}
		return err
	}
	// Ruby sets the query before the opaque, so a conflict names the query.
	for _, i := range []int{0, 1, 2, 3, 5, 7, 6, 8} {
		if err := set(i, c[i]); err != nil {
			return object.Value{}, err
		}
	}
	if c[4] != nil {
		sch := ""
		if p.scheme != nil {
			sch = *p.scheme
		}
		return object.Value{}, ctx.Raise(invalidURI, "the scheme %s does not accept registry part: %s (or bad hostname?)", sch, *c[4])
	}
	if p.path == nil && p.opaque == nil {
		p.path = str("")
	}
	if dp := defaultPort(cls); dp != nil && p.port == nil {
		p.port = dp
	}
	if cls.IsA(ftpClass) && p.path != nil {
		p.path = str(ftpPath(*p.path))
	}
	return build(cls, p), nil
}

// The setters' checks, with Ruby's messages.

func checkScheme(ctx object.Context, v *string) error {
	if v != nil && !schemeRe.MatchString(*v) {
		return ctx.Raise(invalidComp, "bad component(expected scheme component): %s", *v)
	}
	return nil
}

func checkUserinfo(ctx object.Context, p parts, v *string) error {
	if v == nil {
		return nil
	}
	if p.opaque != nil {
		return ctx.Raise(invalidURI, "can not set userinfo with opaque")
	}
	if !userinfoRe.MatchString(*v) {
		return ctx.Raise(invalidComp, "bad component(expected userinfo component): %s", *v)
	}
	return nil
}

func checkHost(ctx object.Context, p parts, v *string) error {
	if v == nil {
		return nil
	}
	if p.opaque != nil {
		return ctx.Raise(invalidURI, "can not set host with registry or opaque")
	}
	if !hostRe.MatchString(*v) {
		return ctx.Raise(invalidComp, "bad component(expected host component): %s", *v)
	}
	return nil
}

func checkPort(ctx object.Context, p parts, v *string) error {
	if v == nil {
		return nil
	}
	if p.opaque != nil {
		return ctx.Raise(invalidURI, "can not set port with registry or opaque")
	}
	if !portRe.MatchString(*v) {
		return ctx.Raise(invalidComp, "bad component(expected port component): %s", object.QuoteString(*v))
	}
	return nil
}

func checkPath(ctx object.Context, p parts, v *string) error {
	if v == nil {
		return nil
	}
	if p.opaque != nil {
		return ctx.Raise(invalidURI, "path conflicts with opaque")
	}
	if *v == "" {
		return nil
	}
	if p.scheme != nil && *p.scheme != "ftp" {
		if !absPathRe.MatchString(*v) {
			return ctx.Raise(invalidComp, "bad component(expected absolute path component): %s", *v)
		}
		return nil
	}
	if !absPathRe.MatchString(*v) && !relPathRe.MatchString(*v) {
		return ctx.Raise(invalidComp, "bad component(expected relative path component): %s", *v)
	}
	return nil
}

func checkOpaque(ctx object.Context, p parts, v *string) error {
	if v == nil {
		return nil
	}
	if p.host != nil || p.port != nil || p.userinfo != nil || p.path != nil && *p.path != "" {
		return ctx.Raise(invalidURI, "can not set opaque with host, port, userinfo or path")
	}
	if !opaqueRe.MatchString(*v) {
		return ctx.Raise(invalidComp, "bad component(expected opaque component): %s", *v)
	}
	return nil
}

// escapeQuery is what Generic#query= does to its value: tabs and line
// ends dropped, a bare % refused, and every character outside the query's
// set percent-encoded.
func escapeQuery(ctx object.Context, v string) (string, error) {
	v = strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(v)
	if badEscapeRe.MatchString(v) {
		return "", ctx.Raise(invalidURI, "invalid percent escape: ")
	}
	return escapeBytes(v, func(c byte) bool {
		return c == '!' || c >= '$' && c <= '&' || c >= '(' && c <= ';' || c == '=' || c >= '?' && c <= '_' || c >= 'a' && c <= '~'
	}), nil
}

// escapeFragment is what Generic#fragment= does: tabs and line ends
// dropped, and every byte outside printable ASCII percent-encoded.
func escapeFragment(v string) string {
	v = strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(v)
	return escapeBytes(v, func(c byte) bool { return c >= '!' && c <= '~' })
}

// escapeBytes percent-encodes every byte keep refuses, leaving a %XX that
// is one already.
func escapeBytes(v string, keep func(byte) bool) string {
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c == '%' && i+2 < len(v) && isHex(v[i+1]) && isHex(v[i+2]) {
			b.WriteString(v[i : i+3])
			i += 2
			continue
		}
		if keep(c) {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// ftpPath is URI::FTP's reading of a path: the leading slash is the
// separator between the authority and the path, not part of it.
func ftpPath(p string) string {
	return strings.TrimPrefix(p, "/")
}

// toS is Generic#to_s.
func toS(v object.Value) string {
	cls := v.Instance().Cls
	if cls.IsA(mailtoClass) {
		return mailtoToS(v)
	}
	p := read(v)
	var b strings.Builder
	if p.scheme != nil {
		b.WriteString(*p.scheme + ":")
	}
	if p.opaque != nil {
		b.WriteString(*p.opaque)
	} else {
		if p.host != nil || p.scheme != nil && (*p.scheme == "file" || *p.scheme == "postgres") {
			b.WriteString("//")
		}
		if p.userinfo != nil {
			b.WriteString(*p.userinfo + "@")
		}
		if p.host != nil {
			b.WriteString(*p.host)
		}
		if p.port != nil && (defaultPort(cls) == nil || *defaultPort(cls) != *p.port) {
			b.WriteString(":" + strconv.FormatInt(*p.port, 10))
		}
		if p.path != nil {
			if cls.IsA(ftpClass) {
				b.WriteString("/" + *p.path)
			} else {
				b.WriteString(*p.path)
			}
		}
		if p.query != nil {
			b.WriteString("?" + *p.query)
		}
	}
	if p.fragment != nil {
		b.WriteString("#" + *p.fragment)
	}
	return b.String()
}

// normalize is Generic#normalize: an empty path becomes "/", and the
// scheme and host are lower-cased.
func normalize(v object.Value) object.Value {
	p := read(v)
	if p.path != nil && *p.path == "" {
		p.path = str("/")
	}
	if p.scheme != nil {
		p.scheme = str(strings.ToLower(*p.scheme))
	}
	if p.host != nil {
		p.host = str(strings.ToLower(*p.host))
	}
	return build(v.Instance().Cls, p)
}

// componentValues is Generic#component_ary: the class's components, read.
func componentValues(ctx object.Context, v object.Value) ([]object.Value, error) {
	names := componentsOf(v.Instance().Cls)
	out := make([]object.Value, len(names))
	for i, n := range names {
		r, err := ctx.Call(v, n)
		if err != nil {
			return nil, err
		}
		out[i] = r
	}
	return out, nil
}

// equal is Generic#==: the same class, and the same components once both
// are normalized.
func equal(ctx object.Context, a, b object.Value) (bool, error) {
	if !isURI(b) || a.Instance().Cls != b.Instance().Cls {
		return false, nil
	}
	x, err := componentValues(ctx, normalize(a))
	if err != nil {
		return false, err
	}
	y, err := componentValues(ctx, normalize(b))
	if err != nil {
		return false, err
	}
	for i := range x {
		eq, err := ctx.Equal(x[i], y[i])
		if err != nil || !eq {
			return false, err
		}
	}
	return true, nil
}

// toURI is the parser's convert_to_uri: a URI as it is, a String parsed,
// anything else refused.
func toURI(ctx object.Context, v object.Value) (object.Value, error) {
	if isURI(v) {
		return v, nil
	}
	if v.IsString() || v.Kind() == object.KindSymbol {
		return parse(ctx, object.String(v.Str()))
	}
	shown, err := ctx.Inspect(v)
	if err != nil {
		return object.Value{}, err
	}
	return object.Value{}, ctx.Raise(invalidURI, "bad URI (is not URI?): %s", shown)
}

// merge is Generic#merge: the reference resolved against the base, as
// Go's net/url resolves it, in the base's class.
func merge(ctx object.Context, base, ref object.Value) (object.Value, error) {
	oth, err := toURI(ctx, ref)
	if err != nil {
		return object.Value{}, err
	}
	o := read(oth)
	if o.scheme != nil {
		return oth, nil
	}
	b := read(base)
	if b.scheme == nil {
		return object.Value{}, ctx.Raise(badURI, "both URI are relative")
	}
	if b.opaque != nil {
		// An opaque base, mailto:x, has no path for a reference to join.
		return base, nil
	}
	bu, err := url.Parse(toS(base))
	if err != nil {
		return object.Value{}, ctx.Raise(badURI, "%s", err.Error())
	}
	ru, err := url.Parse(toS(oth))
	if err != nil {
		return object.Value{}, ctx.Raise(badURI, "%s", err.Error())
	}
	c, err := split(ctx, bu.ResolveReference(ru).String())
	if err != nil {
		return object.Value{}, err
	}
	return construct(ctx, base.Instance().Cls, c, false)
}

func indexOf(ss []string, s string) int {
	for i, x := range ss {
		if x == s {
			return i
		}
	}
	return -1
}

func without(ss []string, s string) []string {
	var out []string
	for _, x := range ss {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

// routeFrom is Generic#route_from: self as a reference relative to oth.
func routeFrom(ctx object.Context, self, other object.Value) (object.Value, error) {
	oth, err := toURI(ctx, other)
	if err != nil {
		return object.Value{}, err
	}
	s, o := read(self), read(oth)
	if s.scheme == nil {
		return object.Value{}, ctx.Raise(badURI, "relative URI: %s", toS(self))
	}
	if o.scheme == nil {
		return object.Value{}, ctx.Raise(badURI, "relative URI: %s", toS(oth))
	}
	if *s.scheme != *o.scheme {
		return self, nil
	}
	rel := s
	rel.scheme = nil
	lower := func(p *string) string {
		if p == nil {
			return ""
		}
		return strings.ToLower(*p)
	}
	if !sameStr(rel.userinfo, o.userinfo) || lower(rel.host) != lower(o.host) || !sameInt(rel.port, o.port) {
		if s.userinfo == nil && s.host == nil {
			return self, nil
		}
		if dp := defaultPort(oth.Instance().Cls); dp != nil && rel.port != nil && *rel.port == *dp {
			rel.port = nil
		}
		return build(genericClass, rel), nil
	}
	rel.userinfo, rel.host, rel.port = nil, nil, nil
	if rel.path != nil && o.path != nil && *rel.path == *o.path {
		rel.path = str("")
		if sameStr(rel.query, o.query) {
			rel.query = nil
		}
		return build(genericClass, rel), nil
	}
	if rel.opaque != nil && o.opaque != nil && *rel.opaque == *o.opaque {
		rel.opaque = str("")
		if sameStr(rel.query, o.query) {
			rel.query = nil
		}
		return build(genericClass, rel), nil
	}
	op, sp := "", ""
	if o.path != nil {
		op = *o.path
	}
	if s.path != nil {
		sp = *s.path
	}
	rel.path = str(routeFromPath(op, sp))
	if *rel.path == "./" && s.query != nil {
		rel.path = str("")
	}
	return build(genericClass, rel), nil
}

func sameStr(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func sameInt(a, b *int64) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

var abnormalPath = regexp.MustCompile(`(?:^|/)\.\.?(?:/|$)`)

// routeFromPath is Generic#route_from_path: dst relative to src.
func routeFromPath(src, dst string) string {
	if dst == src {
		return ""
	}
	if abnormalPath.MatchString(dst) {
		return dst
	}
	srcPath := regexp.MustCompile(`[^/]*/`).FindAllString(src, -1)
	dstPath := scanAll(dst)
	for len(dstPath) > 0 && len(srcPath) > 0 && dstPath[0] == srcPath[0] {
		srcPath, dstPath = srcPath[1:], dstPath[1:]
	}
	tmp := strings.Join(dstPath, "")
	if len(srcPath) == 0 {
		switch {
		case tmp == "":
			return "./"
		case len(dstPath) > 0 && strings.Contains(dstPath[0], ":"):
			return "./" + tmp
		}
		return tmp
	}
	return strings.Repeat("../", len(srcPath)) + tmp
}

// scanAll is dst.scan(%r{[^/]*/?}): the segments with their slash, and one
// empty match for an empty string.
func scanAll(s string) []string {
	out := regexp.MustCompile(`[^/]*/?`).FindAllString(s, -1)
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

// mailto is URI::MailTo's two components, read from the opaque part: the
// addresses, and the headers as pairs.
func mailto(v object.Value) (to string, headers [][2]string) {
	p := read(v)
	if p.opaque == nil {
		return "", nil
	}
	to, hs, found := strings.Cut(*p.opaque, "?")
	if found && hs != "" {
		for _, h := range strings.Split(hs, "&") {
			k, val, _ := strings.Cut(h, "=")
			headers = append(headers, [2]string{k, val})
		}
	}
	return to, headers
}

func mailtoToS(v object.Value) string {
	p := read(v)
	to, headers := mailto(v)
	var b strings.Builder
	b.WriteString("mailto:" + to)
	if len(headers) > 0 {
		b.WriteString("?")
		for i, h := range headers {
			if i > 0 {
				b.WriteString("&")
			}
			b.WriteString(h[0] + "=" + h[1])
		}
	}
	if p.fragment != nil {
		b.WriteString("#" + *p.fragment)
	}
	return b.String()
}

func headersValue(headers [][2]string) object.Value {
	out := make([]object.Value, len(headers))
	for i, h := range headers {
		out[i] = object.NewList(object.String(h[0]), object.String(h[1]))
	}
	return object.NewList(out...)
}

// Components are a URI's parts as another library reads them: net/http
// takes the host, port and request path of a URI::HTTP.
type Components struct {
	Scheme, Userinfo, Host, Path, Query, Fragment string
	Port                                          int64
	HasPort                                       bool
	// RequestURI is the path with the query, "/" for an empty path.
	RequestURI string
}

// Of reads a URI::Generic, and false for any other value.
func Of(v object.Value) (Components, bool) {
	if !isURI(v) {
		return Components{}, false
	}
	p := read(v)
	get := func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	}
	c := Components{Scheme: get(p.scheme), Userinfo: get(p.userinfo), Host: get(p.host), Path: get(p.path), Query: get(p.query), Fragment: get(p.fragment)}
	if p.port != nil {
		c.Port, c.HasPort = *p.port, true
	}
	c.RequestURI = c.Path
	if p.query != nil {
		c.RequestURI += "?" + *p.query
	}
	if !strings.HasPrefix(c.RequestURI, "/") {
		c.RequestURI = "/" + c.RequestURI
	}
	return c, true
}

// Parse is URI.parse for another library: the URI of s, or
// URI::InvalidURIError.
func Parse(ctx object.Context, s string) (object.Value, error) {
	return parse(ctx, object.String(s))
}

// Encode is URI.encode_www_form_component.
func Encode(s string) string { return encodeComponent(s, true) }
