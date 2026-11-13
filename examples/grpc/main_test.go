package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

// serve starts a hello.HelloService on a port of this machine, as
// grpcb.in answers it: the reply is "hello " and the greeting. It reads
// and writes the messages by the descriptors of the script's classes, and
// refuses the greeting "nobody". It returns a connection to the service.
func serve(t *testing.T) *grpc.ClientConn {
	t.Helper()
	prog, err := cameo.Load("hello.cameo", script, io.Discard, io.Discard)
	assert.NoError(t, err)
	assert.NoError(t, prog.Run())
	request, err := prog.Descriptor("Hello::HelloRequest")
	assert.NoError(t, err)
	response, err := prog.Descriptor("Hello::HelloResponse")
	assert.NoError(t, err)
	field := func(md protoreflect.MessageDescriptor, name string) protoreflect.FieldDescriptor {
		return md.Fields().ByName(protoreflect.Name(name))
	}
	srv := grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		method, _ := grpc.MethodFromServerStream(stream)
		if method != "/hello.HelloService/SayHello" {
			return status.Errorf(codes.Unimplemented, "no method %s", method)
		}
		in := dynamicpb.NewMessage(request)
		if err := stream.RecvMsg(in); err != nil {
			return err
		}
		greeting := in.Get(field(request, "greeting")).String()
		if greeting == "nobody" {
			return status.Error(codes.InvalidArgument, "nobody to greet")
		}
		out := dynamicpb.NewMessage(response)
		out.Set(field(response, "reply"), protoreflect.ValueOfString("hello "+greeting))
		return stream.SendMsg(out)
	}))
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	assert.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

// The request a script builds is the message the service reads, and the
// service's answer is an instance of the script's class.
func TestTheScriptCallsTheService(t *testing.T) {
	var out bytes.Buffer
	replies, err := greetAll(context.Background(), serve(t), []string{"Ada", "Grace"}, &out, &out)
	assert.NoError(t, err)
	assert.Equals(t, replies, []any{"hello Ada", "hello Grace"})
	assert.Equals(t, out.String(), "")
}

// An error of the service is an exception the script rescues.
func TestTheScriptRescuesAFailureOfTheService(t *testing.T) {
	var out bytes.Buffer
	replies, err := greetAll(context.Background(), serve(t), []string{"Ada", "nobody"}, &out, &out)
	assert.NoError(t, err)
	assert.Equals(t, replies, []any{"the service failed: rpc error: code = InvalidArgument desc = nobody to greet"})
}
