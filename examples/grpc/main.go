// Command grpc calls a gRPC service from a cameo script. A class of a
// script is a protobuf message, so the request the script builds goes on
// the wire as it is, and the response comes back as an instance of a
// class of the script: no generated code and no conversion.
//
// The service is hello.HelloService of grpcb.in, a public test server
// that anyone may call:
//
//	go run . Ada Grace              # in examples/grpc
//	go run . -addr localhost:9000 -plaintext Ada
//
// cameo does no I/O, so the Go program gives the script a module,
// HelloService, whose say_hello starts the call on a goroutine and
// returns a promise; the script awaits it.
package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/cameo-lang/cameo/src/cameo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
)

//go:embed hello.cameo
var script string

// greetAll runs the script's greet_all over names, with conn as the
// connection to the service, and returns the replies.
func greetAll(ctx context.Context, conn grpc.ClientConnInterface, names []string, stdout, stderr io.Writer) ([]any, error) {
	prog, err := cameo.Load("hello.cameo", script, stdout, stderr)
	if err != nil {
		return nil, err
	}
	service := prog.Module("HelloService")
	failed := service.Exception("Failed")
	service.Def("say_hello", func(args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
		if len(args) != 1 {
			return nil, fmt.Errorf("say_hello takes one argument, not %d", len(args))
		}
		// An instance of a class of the script is a proto.Message here.
		request, ok := args[0].(proto.Message)
		if !ok {
			return nil, fmt.Errorf("say_hello takes a Hello::HelloRequest, not %T", args[0])
		}
		// The class the script declares for the response, as a descriptor:
		// the answer is read into a message of it. The script has defined
		// the class by the time it calls this.
		response, err := prog.Descriptor("Hello::HelloResponse")
		if err != nil {
			return nil, err
		}
		promise := prog.Promise()
		go func() {
			reply := dynamicpb.NewMessage(response)
			if err := conn.Invoke(ctx, "/hello.HelloService/SayHello", request, reply); err != nil {
				promise.Fail(failed.New("%v", err))
				return
			}
			// The message becomes a Hello::HelloResponse in the script.
			promise.Resolve(reply)
		}()
		return promise, nil
	})
	// Run defines the script's classes and methods.
	if err := prog.Run(); err != nil {
		return nil, err
	}
	replies, err := prog.Call(nil, "greet_all", names)
	if err != nil {
		return nil, err
	}
	return replies.([]any), nil
}

func main() {
	addr := flag.String("addr", "grpcb.in:9001", "the address of a hello.HelloService")
	plaintext := flag.Bool("plaintext", false, "connect without TLS, as grpcb.in:9000 takes it")
	flag.Parse()
	names := flag.Args()
	if len(names) == 0 {
		names = []string{"cameo"}
	}
	creds := credentials.NewClientTLSFromCert(nil, "")
	if *plaintext {
		creds = insecure.NewCredentials()
	}
	conn, err := grpc.NewClient(*addr, grpc.WithTransportCredentials(creds))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	replies, err := greetAll(ctx, conn, names, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, reply := range replies {
		fmt.Println(reply)
	}
}
