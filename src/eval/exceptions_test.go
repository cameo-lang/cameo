package eval

import "testing"

const httpError = `
class HTTPError < StandardError
  int :status
  string :url, default: ""

  def initialize(status, url: "")
    super("HTTP #{status}")
    self.status = status
    self.url = url
  end
end
`

func TestAnExceptionsFieldsAreSetInInitializeAndReadAfterRescue(t *testing.T) {
	expectOutput(t, httpError+`
begin
  raise HTTPError.new(404, url: "/x")
rescue HTTPError => e
  p e.status, e.url, e.message, e
end
`, "404\n\"/x\"\n\"HTTP 404\"\n#<HTTPError: HTTP 404>\n")
}

func TestSuperFromInitializeSetsTheMessage(t *testing.T) {
	expectOutput(t, `
class E < StandardError
  def initialize(n)
    super("n is #{n}")
  end
end
class F < StandardError
  def initialize(n)
    super
  end
end
class G < StandardError
  def initialize(n)
    super()
  end
end
p E.new(1).message, F.new(2).message, G.new(3).message
`, "\"n is 1\"\n\"2\"\n\"G\"\n")
}

func TestAnInitializeWithoutSuperLeavesTheClassNameAsTheMessage(t *testing.T) {
	expectOutput(t, `
class E < StandardError
  int :code
  def initialize(code)
    self.code = code
  end
end
e = E.new(7)
p e.message, e.code
`, "\"E\"\n7\n")
}

func TestRaiseWithAClassAndAMessageRunsInitialize(t *testing.T) {
	expectOutput(t, `
class E < StandardError
  string :seen
  def initialize(msg = "none")
    super("got #{msg}")
    self.seen = msg
  end
end
begin
  raise E, "x"
rescue E => e
  p e.message, e.seen
end
begin
  raise E
rescue E => e
  p e.message
end
`, "\"got x\"\n\"x\"\n\"got none\"\n")
}

func TestAnExceptionWithoutInitializeTakesItsFieldsAsKeywords(t *testing.T) {
	expectOutput(t, `
class E < StandardError
  int :status
end
p E.new(status: 1).message, E.new(status: 1).status
e = E.new("broke", status: 2)
p e.message, e.status
begin
  raise E, "plain"
rescue E => x
  p x.message, x.status
end
`, "\"E\"\n1\n\"broke\"\n2\n\"plain\"\nnil\n")
	expectError(t, "class E < StandardError\n  int :status\nend\nE.new(stauts: 1)\n", "E.new: unknown field stauts (ArgumentError)")
	expectError(t, "class E < StandardError\n  int :status\nend\nE.new(status: \"x\")\n", "field status of E takes an int")
}

func TestWithCopiesAnExceptionWithItsMessageAndFields(t *testing.T) {
	expectOutput(t, httpError+`
e = HTTPError.new(404)
f = e.with(status: 500, message: "server")
p f, f.status, e.status, e.message
p e == HTTPError.new(404), e == f
`, "#<HTTPError: server>\n500\n404\n\"HTTP 404\"\ntrue\nfalse\n")
	expectError(t, httpError+"HTTPError.new(1).with(nope: 1)\n", "HTTPError has no field nope (NameError)")
}

func TestAnExceptionOfAScriptClassReachesTheTopLevelWithItsLine(t *testing.T) {
	expectError(t, httpError+"\nraise HTTPError.new(503)\n", "t.cameo:13: HTTP 503 (HTTPError)")
}

func TestAnExceptionKeepsItsFieldsAfterInitialize(t *testing.T) {
	expectError(t, httpError+"e = HTTPError.new(1)\ne.send(:initialize, 2)\n", "initialize runs only in new")
	expectError(t, "e = StandardError.new(\"a\")\ne.initialize(\"b\")\n", "initialize runs only in new (CameoError)")
}

func TestAnExceptionClassKeepsItsOwnPartsFromFields(t *testing.T) {
	expectError(t, "class E < StandardError\n  string :message\nend\n", "field message: an exception of E carries its message already")
	expectError(t, "class E < KeyError\n  string :key\nend\n", "field key: an exception of E carries its key already")
	expectError(t, "class E < StandardError\n  int :status, number: 3\nend\n", "field status: numbers 1 to 15 are the exception's own")
	expectError(t, "class E < StandardError\n  reserved 2\nend\n", "reserved 2: numbers 1 to 15 are the exception's own")
}

func TestARaiseInARescueGetsTheRescuedExceptionAsItsCause(t *testing.T) {
	expectOutput(t, `
begin
  begin
    raise "inner"
  rescue => e
    raise ArgumentError, "outer"
  end
rescue => o
  p o.cause
end
begin
  begin
    raise "a"
  rescue
    raise
  end
rescue => r
  p r, r.cause
end
begin
  raise "b", cause: nil
rescue => n
  p n.cause
end
begin
  raise "c", cause: TypeError.new("t")
rescue => c
  p c.cause
end
`, "#<RuntimeError: inner>\n#<RuntimeError: a>\nnil\nnil\n#<TypeError: t>\n")
}

func TestToProtoCarriesAnExceptionsParts(t *testing.T) {
	expectOutput(t, httpError+`
e = HTTPError.from_proto(HTTPError.new(418, url: "/tea").to_proto)
p e, e.status, e.url
k = begin
  {a: 1}.fetch(:b)
rescue KeyError => x
  x
end
y = from_value_proto(k.to_value_proto)
p y, y.key, y.receiver
`, "#<HTTPError: HTTP 418>\n418\n\"/tea\"\n#<KeyError: key not found: :b>\n\"b\"\n{\"a\" => 1}\n")
}

func TestAnAsyncWorkersExceptionReachesAwaitWithItsFields(t *testing.T) {
	expectOutput(t, "include Cameo\n"+httpError+`
class Fetcher
  def get(url)
    raise HTTPError.new(503, url: url) if url.start_with?("/down")
    url
  end
end
begin
  await Fetcher.new, :get, "/down/1"
rescue HTTPError => e
  p e, e.status, e.url
end
p await(Fetcher.new, :get, "/up")
pr = promise
fail pr, HTTPError, 418
begin
  await pr
rescue HTTPError => e
  p e.status
end
`, "#<HTTPError: HTTP 503>\n503\n\"/down/1\"\n\"/up\"\n418\n")
}
