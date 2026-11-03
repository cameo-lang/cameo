package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	gohttp "net/http"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
)

// host is what the interpreter gives a request: the client the Go host
// set, and the channel a stopped run closes. eval.Interp implements it.
type host interface {
	HTTPClient() *gohttp.Client
	Done() <-chan struct{}
}

// perform sends a request over a connection and returns the response, or
// raises as Ruby's Net::HTTP does for what went wrong.
func perform(ctx object.Context, c conn, req *request) (object.Value, error) {
	h, ok := ctx.(host)
	var client *gohttp.Client
	if ok {
		client = h.HTTPClient()
	}
	where := fmt.Sprintf("%s:%d", c.address, c.port)
	if client == nil {
		return object.Value{}, ctx.Raise(socketError, "Failed to open TCP connection to %s (the program has no network access: its host gave it no HTTP client)", where)
	}
	scheme := "http"
	if c.useSSL {
		scheme = "https"
	}
	url := scheme + "://" + net.JoinHostPort(c.address, strconv.FormatInt(c.port, 10)) + req.path
	var body io.Reader
	if req.body != nil {
		body = strings.NewReader(*req.body)
	}
	bg := context.Background()
	var cancel context.CancelFunc = func() {}
	if c.readTimeout > 0 {
		bg, cancel = context.WithTimeout(bg, time.Duration(c.readTimeout*float64(time.Second)))
	}
	defer cancel()
	if ok {
		// A stopped run cancels the request.
		var stop context.CancelFunc
		bg, stop = context.WithCancel(bg)
		defer stop()
		go func() {
			select {
			case <-h.Done():
				stop()
			case <-bg.Done():
			}
		}()
	}
	greq, err := gohttp.NewRequestWithContext(bg, req.method, url, body)
	if err != nil {
		return object.Value{}, ctx.Raise(object.ArgumentError, "%v", err)
	}
	for _, name := range req.headers.names {
		value := req.headers.values[name]
		if name == "host" {
			greq.Host = value
			continue
		}
		greq.Header.Set(name, value)
	}
	if req.body != nil {
		greq.ContentLength = int64(len(*req.body))
		// Ruby supplies a form content type for a body sent without one.
		if !req.headers.Has("content-type") {
			greq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	// Ruby does not follow redirects; the script sees the 3xx response.
	cl := *client
	cl.CheckRedirect = func(*gohttp.Request, []*gohttp.Request) error { return gohttp.ErrUseLastResponse }
	resp, err := cl.Do(greq)
	if err != nil {
		return object.Value{}, raise(ctx, err, c, where)
	}
	defer resp.Body.Close()
	res := &response{cls: classForCode(resp.StatusCode), code: resp.StatusCode, version: fmt.Sprintf("%d.%d", resp.ProtoMajor, resp.ProtoMinor), headers: newHeaders(), body: object.Nil}
	res.message = strings.TrimPrefix(resp.Status, strconv.Itoa(resp.StatusCode)+" ")
	for _, name := range canonicalOrder(resp.Header) {
		res.headers.Set(name, strings.Join(resp.Header.Values(name), ", "))
	}
	if req.uri != nil {
		res.uri = uriString(*req.uri)
	} else {
		res.uri = url
	}
	hasBody := req.method != "HEAD" && resp.StatusCode != 204 && resp.StatusCode != 304 && resp.StatusCode/100 != 1
	if hasBody {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return object.Value{}, raise(ctx, err, c, where)
		}
		// Ruby reads the body into a UTF-8 String; one that is not UTF-8
		// stays binary here, where Ruby's would hold the bytes and fail
		// on the first character method.
		if utf8.Valid(data) {
			res.body = object.String(string(data))
		} else {
			res.body = object.Bytes(data)
		}
	}
	return res.value(), nil
}

// canonicalOrder lists a Go header's names in a stable order.
func canonicalOrder(h gohttp.Header) []string {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	// Go's Header is a map; sort for a stable each_header.
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j] < names[j-1]; j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	return names
}

// raise turns Go's error into the exception Ruby's Net::HTTP raises for
// the same failure.
func raise(ctx object.Context, err error, c conn, where string) error {
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return ctx.Raise(socketError, "Failed to open TCP connection to %s (getaddrinfo: Name or service not known)", where)
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return ctx.Raise(econnrefused, "Failed to open TCP connection to %s (Connection refused - connect(2) for %s port %d)", where, object.QuoteString(c.address), c.port)
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return ctx.Raise(econnreset, "Connection reset by peer")
	}
	if errors.Is(err, syscall.EHOSTUNREACH) {
		return ctx.Raise(ehostunreach, "Failed to open TCP connection to %s (No route to host - connect(2) for %s port %d)", where, object.QuoteString(c.address), c.port)
	}
	if errors.Is(err, syscall.ENETUNREACH) {
		return ctx.Raise(enetunreach, "Failed to open TCP connection to %s (Network is unreachable - connect(2) for %s port %d)", where, object.QuoteString(c.address), c.port)
	}
	var certErr *tls.CertificateVerificationError
	var hostErr x509.HostnameError
	var unknownAuth x509.UnknownAuthorityError
	if errors.As(err, &certErr) || errors.As(err, &hostErr) || errors.As(err, &unknownAuth) || strings.Contains(err.Error(), "tls:") {
		return ctx.Raise(sslError, "SSL_connect returned=1 errno=0 peeraddr=%s state=error: certificate verify failed (%v)", where, unwrapURL(err))
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) && netErr.Timeout() {
		return ctx.Raise(readTimeout, "Net::ReadTimeout with #<TCPSocket:(closed)>")
	}
	if errors.Is(err, context.Canceled) {
		return ctx.Raise(object.Interrupt, "")
	}
	return ctx.Raise(socketError, "%v", unwrapURL(err))
}

// unwrapURL takes the "Get \"url\": " prefix off a url.Error.
func unwrapURL(err error) error {
	for {
		u := errors.Unwrap(err)
		if u == nil {
			return err
		}
		if _, isURL := err.(interface{ Timeout() bool }); !isURL {
			return err
		}
		err = u
	}
}

// quietContext is the Context the library parses a URI with where no
// interpreter is at hand: it raises into a plain error.
type quietContext struct{}

func (quietContext) Call(recv object.Value, name string, args ...object.Value) (object.Value, error) {
	return object.Value{}, errors.New("no interpreter")
}
func (quietContext) CallWith(recv object.Value, name string, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
	return object.Value{}, errors.New("no interpreter")
}
func (quietContext) Yield(block object.Value, args ...object.Value) (object.Value, error) {
	return object.Value{}, errors.New("no interpreter")
}
func (quietContext) Block(f func(args []object.Value) (object.Value, error)) object.Value {
	return object.Nil
}
func (quietContext) Raise(cls *object.Class, format string, args ...any) error {
	return &object.Exception{Cls: cls, Message: fmt.Sprintf(format, args...)}
}
func (quietContext) Inspect(v object.Value) (string, error) { return object.Key(v), nil }
func (quietContext) ToS(v object.Value) (string, error)     { return v.Str(), nil }
func (quietContext) Equal(a, b object.Value) (bool, error) {
	return object.Key(a) == object.Key(b), nil
}
func (quietContext) Compare(a, b object.Value) (int, error) {
	return strings.Compare(object.Key(a), object.Key(b)), nil
}
func (quietContext) Write(stderr bool, s string) error { return nil }
func (quietContext) RespondsTo(v object.Value, name string) bool {
	return v.Class().Lookup(name) != nil
}
func (quietContext) Rand() *object.Random                   { return nil }
func (quietContext) Const(name string) (object.Value, bool) { return object.Value{}, false }

// protogenNumber is protogen.FieldNumber, named here so request.go reads
// without the import.
func protogenNumber(name string, taken func(int64) bool) int64 {
	return protogen.FieldNumber(name, taken)
}
