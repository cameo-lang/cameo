package http

import (
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/lib/uri"
	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

// Net::HTTPResponse and its subclasses by status, as Ruby has them: a
// response is an instance of the class of its status code, so `case res
// when Net::HTTPSuccess` works. The fields are the code, the message, the
// HTTP version, the headers, the body (a String; binary when it is not
// UTF-8, and nil where a response has none) and the URI requested.

var (
	responseClass   = object.NewClass("HTTPResponse", object.ObjectClass)
	responseClasses = map[int]*object.Class{}
	// responseRanges are the classes of a range of codes, by first digit.
	responseRanges  = map[int]*object.Class{}
	unknownResponse *object.Class
)

const (
	sCode = iota
	sMessage
	sVersion
	sHeaders
	sBody
	sURI
	sFieldCount
)

var statusClasses = []struct {
	name string
	code int
}{
	{"HTTPContinue", 100}, {"HTTPSwitchProtocol", 101}, {"HTTPProcessing", 102}, {"HTTPEarlyHints", 103},
	{"HTTPOK", 200}, {"HTTPCreated", 201}, {"HTTPAccepted", 202}, {"HTTPNonAuthoritativeInformation", 203}, {"HTTPNoContent", 204},
	{"HTTPResetContent", 205}, {"HTTPPartialContent", 206}, {"HTTPMultiStatus", 207}, {"HTTPAlreadyReported", 208}, {"HTTPIMUsed", 226},
	{"HTTPMultipleChoices", 300}, {"HTTPMovedPermanently", 301}, {"HTTPFound", 302}, {"HTTPSeeOther", 303}, {"HTTPNotModified", 304},
	{"HTTPUseProxy", 305}, {"HTTPTemporaryRedirect", 307}, {"HTTPPermanentRedirect", 308},
	{"HTTPBadRequest", 400}, {"HTTPUnauthorized", 401}, {"HTTPPaymentRequired", 402}, {"HTTPForbidden", 403}, {"HTTPNotFound", 404},
	{"HTTPMethodNotAllowed", 405}, {"HTTPNotAcceptable", 406}, {"HTTPProxyAuthenticationRequired", 407}, {"HTTPRequestTimeout", 408},
	{"HTTPConflict", 409}, {"HTTPGone", 410}, {"HTTPLengthRequired", 411}, {"HTTPPreconditionFailed", 412}, {"HTTPPayloadTooLarge", 413},
	{"HTTPURITooLong", 414}, {"HTTPUnsupportedMediaType", 415}, {"HTTPRangeNotSatisfiable", 416}, {"HTTPExpectationFailed", 417},
	{"HTTPMisdirectedRequest", 421}, {"HTTPUnprocessableEntity", 422}, {"HTTPLocked", 423}, {"HTTPFailedDependency", 424},
	{"HTTPTooEarly", 425}, {"HTTPUpgradeRequired", 426}, {"HTTPPreconditionRequired", 428}, {"HTTPTooManyRequests", 429},
	{"HTTPRequestHeaderFieldsTooLarge", 431}, {"HTTPUnavailableForLegalReasons", 451},
	{"HTTPInternalServerError", 500}, {"HTTPNotImplemented", 501}, {"HTTPBadGateway", 502}, {"HTTPServiceUnavailable", 503},
	{"HTTPGatewayTimeout", 504}, {"HTTPVersionNotSupported", 505}, {"HTTPVariantAlsoNegotiates", 506}, {"HTTPInsufficientStorage", 507},
	{"HTTPLoopDetected", 508}, {"HTTPNotExtended", 510}, {"HTTPNetworkAuthenticationRequired", 511},
}

var rangeClasses = []struct {
	name  string
	digit int
}{
	{"HTTPInformation", 1}, {"HTTPSuccess", 2}, {"HTTPRedirection", 3}, {"HTTPClientError", 4}, {"HTTPServerError", 5},
}

// response is a response as Go reads it.
type response struct {
	cls     *object.Class
	code    int
	message string
	version string
	headers *headers
	body    object.Value // Bytes, or Nil
	uri     string
}

func (r *response) value() object.Value {
	fields := make([]object.Value, len(r.cls.Fields))
	fields[sCode] = object.String(strconv.Itoa(r.code))
	fields[sMessage] = object.String(r.message)
	fields[sVersion] = object.String(r.version)
	fields[sHeaders] = r.headers.toMap()
	fields[sBody] = r.body
	fields[sURI] = object.Nil
	if r.uri != "" {
		fields[sURI] = object.String(r.uri)
	}
	return object.InstanceValue(&object.Instance{Cls: r.cls, Fields: fields})
}

func readResponse(v object.Value) *response {
	inst := v.Instance()
	f := inst.Fields
	code, _ := strconv.Atoi(f[sCode].Str())
	r := &response{cls: inst.Cls, code: code, message: f[sMessage].Str(), version: f[sVersion].Str(), headers: headersOf(f[sHeaders]), body: f[sBody]}
	if !f[sURI].IsNil() {
		r.uri = f[sURI].Str()
	}
	return r
}

// classForCode is the response class of a status code: its own, its
// range's, or HTTPUnknownResponse.
func classForCode(code int) *object.Class {
	if c, ok := responseClasses[code]; ok {
		return c
	}
	if c, ok := responseRanges[code/100]; ok {
		return c
	}
	return unknownResponse
}

// exceptionFor is the error class a response's value raises: by range,
// as Ruby's EXCEPTION_TYPE has it.
func exceptionFor(code int) *object.Class {
	switch code / 100 {
	case 1:
		return httpError
	case 3:
		return httpRetriableError
	case 4:
		return httpClientExc
	case 5:
		return httpFatalError
	}
	return httpError
}

func installResponses() {
	c := responseClass
	c.Lexical = netModule
	netModule.Consts[c.Name] = object.ClassValue(c)
	declare(c, []struct{ name, typ string }{{"code", "string"}, {"message", "string"}, {"http_version", "string"}})
	c.FieldIndex["headers"] = len(c.Fields)
	c.Fields = append(c.Fields, mapField(c, "headers"))
	declare(c, []struct{ name, typ string }{{"body", "bytes"}, {"uri", "string"}})
	c.Include(headerModule)
	for _, r := range rangeClasses {
		rc := object.NewClass(r.name, c)
		rc.Lexical = netModule
		netModule.Consts[r.name] = object.ClassValue(rc)
		inherit(rc)
		responseRanges[r.digit] = rc
	}
	unknownResponse = object.NewClass("HTTPUnknownResponse", c)
	unknownResponse.Lexical = netModule
	netModule.Consts["HTTPUnknownResponse"] = object.ClassValue(unknownResponse)
	inherit(unknownResponse)
	codeToObj := object.NewMapBuilder()
	for _, s := range statusClasses {
		sc := object.NewClass(s.name, responseRanges[s.code/100])
		sc.Lexical = netModule
		netModule.Consts[s.name] = object.ClassValue(sc)
		inherit(sc)
		sc.Consts["HAS_BODY"] = object.Bool(s.code != 204 && s.code != 304 && s.code/100 != 1)
		responseClasses[s.code] = sc
		codeToObj.Set(object.String(strconv.Itoa(s.code)), object.ClassValue(sc))
	}
	netModule.Consts["HTTPRequestURITooLong"] = object.ClassValue(responseClasses[414])
	netModule.Consts["HTTPRequestEntityTooLarge"] = object.ClassValue(responseClasses[413])
	netModule.Consts["HTTPRequestedRangeNotSatisfiable"] = object.ClassValue(responseClasses[416])
	netModule.Consts["HTTPMovedTemporarily"] = object.ClassValue(responseClasses[302])
	netModule.Consts["HTTPClientErrorCode"] = object.ClassValue(responseRanges[4])
	netModule.Consts["HTTPServerErrorCode"] = object.ClassValue(responseRanges[5])
	netModule.Consts["HTTPSuccessCode"] = object.ClassValue(responseRanges[2])
	netModule.Consts["HTTPRedirectionCode"] = object.ClassValue(responseRanges[3])
	netModule.Consts["HTTPInformationCode"] = object.ClassValue(responseRanges[1])
	netModule.Consts["HTTPResponseReceiver"] = object.ClassValue(c)
	c.Consts["CODE_TO_OBJ"] = object.MapValue(codeToObj.Map())
	c.Consts["CODE_CLASS_TO_OBJ"] = func() object.Value {
		b := object.NewMapBuilder()
		for _, r := range rangeClasses {
			b.Set(object.String(strconv.Itoa(r.digit)), object.ClassValue(responseRanges[r.digit]))
		}
		return object.MapValue(b.Map())
	}()
	for _, rc := range responseRanges {
		rc.Consts["HAS_BODY"] = object.Bool(rc != responseRanges[1])
	}
	c.DefineSingleton("body_permitted?", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		v, ok := self.Cls().Consts["HAS_BODY"]
		return object.Bool(!ok || v.Truthy()), nil
	})
	c.DefineSingleton("exception_type", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		for code, rc := range responseRanges {
			if self.Cls().IsA(rc) {
				return object.ClassValue(exceptionFor(code * 100)), nil
			}
		}
		return object.ClassValue(httpError), nil
	})

	reader := func(name string, f func(r *response) object.Value) {
		c.Define(name, 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(readResponse(self)), nil
		})
	}
	reader("code", func(r *response) object.Value { return object.String(strconv.Itoa(r.code)) })
	reader("message", func(r *response) object.Value { return object.String(r.message) })
	reader("msg", func(r *response) object.Value { return object.String(r.message) })
	reader("http_version", func(r *response) object.Value { return object.String(r.version) })
	reader("body", func(r *response) object.Value { return r.body })
	reader("read_body", func(r *response) object.Value { return r.body })
	reader("entity", func(r *response) object.Value { return r.body })
	reader("code_type", func(r *response) object.Value { return object.ClassValue(r.cls) })
	reader("response", func(r *response) object.Value { return r.value() })
	reader("decode_content", func(r *response) object.Value { return object.True })
	reader("body_encoding", func(r *response) object.Value { return object.False })
	reader("uri", func(r *response) object.Value {
		if r.uri == "" {
			return object.Nil
		}
		v, err := uri.Parse(quietContext{}, r.uri)
		if err != nil {
			return object.Nil
		}
		return v
	})
	reader("inspect", func(r *response) object.Value {
		return object.String("#<" + r.cls.QualifiedName() + " " + strconv.Itoa(r.code) + " " + r.message + " readbody=true>")
	})
	reader("to_ary", func(r *response) object.Value { return object.Nil })
	// value raises unless the response is a success; error! raises always.
	errorFn := func(ctx object.Context, r *response) error {
		cls := exceptionFor(r.code)
		exc := &object.Exception{Cls: cls, Message: strconv.Itoa(r.code) + " " + object.QuoteString(r.message), Attrs: map[string]object.Value{"response": r.value()}}
		return exc
	}
	c.Define("value", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		r := readResponse(self)
		if r.code/100 == 2 {
			return object.Nil, nil
		}
		return object.Value{}, errorFn(ctx, r)
	})
	c.Define("error!", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Value{}, errorFn(ctx, readResponse(self))
	})
	c.Define("error_type", 0, func(ctx object.Context, self object.Value, a []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.ClassValue(exceptionFor(readResponse(self).code)), nil
	})
	installHeaderMethods(c, func(v object.Value) *headers {
		return readResponse(v).headers
	}, func(v object.Value, h *headers) object.Value {
		r := readResponse(v)
		r.headers = h
		return r.value()
	})
	_ = stdlib.ArgError
	_ = strings.ToLower
}
