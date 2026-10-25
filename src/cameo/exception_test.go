package cameo

import (
	"bytes"
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const httpErrorScript = `
class HTTPError < StandardError
  int :status
  def initialize(status)
    super("HTTP #{status}")
    self.status = status
  end
end

def fetch(status) = raise HTTPError.new(status)
def caught(status)
  fetch(status)
rescue HTTPError => e
  e
end
`

// field reads a field of a message by name.
func field(t *testing.T, msg proto.Message, name string) protoreflect.Value {
	t.Helper()
	m := msg.ProtoReflect()
	fd := m.Descriptor().Fields().ByName(protoreflect.Name(name))
	if fd == nil {
		t.Fatalf("%s has no field %s", m.Descriptor().FullName(), name)
	}
	return m.Get(fd)
}

func TestARunErrorGivesTheHostTheExceptionAsAMessage(t *testing.T) {
	var out bytes.Buffer
	prog, err := Load("t.cameo", httpErrorScript+"fetch(404)\n", &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	err = prog.Run()
	var runErr *RunError
	if !errors.As(err, &runErr) {
		t.Fatalf("Run returned %v", err)
	}
	msg := runErr.Exception()
	if msg == nil {
		t.Fatal("no exception in the RunError")
	}
	if name := msg.ProtoReflect().Descriptor().FullName(); name != "cameo.HTTPError" {
		t.Fatalf("the exception is a %s", name)
	}
	if got := field(t, msg, "message").String(); got != "HTTP 404" {
		t.Fatalf("message %q", got)
	}
	if got := field(t, msg, "status").Int(); got != 404 {
		t.Fatalf("status %d", got)
	}
	if got := field(t, msg, "line").Int(); got != 10 {
		t.Fatalf("line %d", got)
	}
}

func TestARunErrorOfABuiltinExceptionCarriesWhatItCarries(t *testing.T) {
	var out bytes.Buffer
	prog, err := Load("t.cameo", "{a: 1}.fetch(:b)\n", &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	var runErr *RunError
	if !errors.As(prog.Run(), &runErr) {
		t.Fatal("the script did not fail")
	}
	msg := runErr.Exception()
	if name := msg.ProtoReflect().Descriptor().FullName(); name != "cameo.KeyError" {
		t.Fatalf("the exception is a %s", name)
	}
	key := field(t, msg, "key").Message()
	if got := key.Get(key.Descriptor().Fields().ByName("string_value")).String(); got != "b" {
		t.Fatalf("key %q", got)
	}
}

func TestCallReturnsAnExceptionAsAMessage(t *testing.T) {
	var out bytes.Buffer
	prog, err := Load("t.cameo", httpErrorScript, &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	got, err := prog.Call(nil, "caught", 503)
	if err != nil {
		t.Fatal(err)
	}
	msg, ok := got.(proto.Message)
	if !ok {
		t.Fatalf("an exception comes back as a proto.Message, got %T", got)
	}
	if field(t, msg, "status").Int() != 503 || field(t, msg, "message").String() != "HTTP 503" {
		t.Fatalf("exception %v", msg)
	}
	// The message goes back in as the exception it is.
	back, err := prog.ToValue(msg)
	if err != nil {
		t.Fatal(err)
	}
	if shown, _ := prog.in.Inspect(back); shown != "#<HTTPError: HTTP 503>" {
		t.Fatalf("back in cameo: %s", shown)
	}
}

// TestAHostExceptionGoesBackInAsItself proves the message of an exception
// class a host module defines converts back into that exception, as a
// script's or a builtin's does.
func TestAHostExceptionGoesBackInAsItself(t *testing.T) {
	var out bytes.Buffer
	prog, err := Load("t.cameo", "def caught\n  Net.get\nrescue Net::Timeout => e\n  e\nend\n", &out, &out)
	if err != nil {
		t.Fatal(err)
	}
	net := prog.Module("Net")
	timeout := net.Exception("Timeout")
	net.Def("get", func([]any, map[string]any) (any, error) { return nil, timeout.New("too slow") })
	got, err := prog.Call(nil, "caught")
	if err != nil {
		t.Fatal(err)
	}
	msg, ok := got.(proto.Message)
	if !ok {
		t.Fatalf("an exception comes back as a proto.Message, got %T", got)
	}
	back, err := prog.ToValue(msg)
	if err != nil {
		t.Fatal(err)
	}
	if shown, _ := prog.in.Inspect(back); shown != "#<Net::Timeout: too slow>" {
		t.Fatalf("back in cameo: %s", shown)
	}
}
