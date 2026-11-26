## Calling a gRPC service {#ch-grpc}

::: source
from `doc/gh_pages/grpc.html`
:::

A class of a script is a protobuf message. A script builds the request of a
gRPC call as an instance of its own class, and the response comes back as one:
no generated code, no conversion.

1.  [Run it](#grpc-run)
2.  [The script](#grpc-script)
3.  [How the classes match the service](#grpc-match)
4.  [The Go program](#grpc-go)

The example is
[examples/grpc](https://github.com/cameo-lang/cameo/tree/main/examples/grpc).
It calls `hello.HelloService` of [grpcb.in](https://grpcb.in), a public test
server ([moul/grpcbin](https://github.com/moul/grpcbin)) that anyone may call
without a key.

### Run it {#grpc-run}

``` sh
cd examples/grpc
go run . Ada Grace
```

``` output
hello Ada
hello Grace
```

The default address is `grpcb.in:9001`, with TLS;
`go run . -addr grpcb.in:9000 -plaintext Ada` goes without. The example's test
runs the same script against a service on its own machine, so it needs no
network.

### The script {#grpc-script}

The script declares the service's two messages as classes, sends one request
per name, and awaits the answers. Every call is on its way before the first
answer is read.

``` {.cameo .hosted file="examples/grpc/hello.cameo" data-title="hello.cameo"}
# The messages of grpcb.in's hello.HelloService, declared as classes. A
# class in the module Hello is the message hello.<name>, and number: sets
# the field's number, so an instance is on the wire what the service
# reads and writes.
module Hello
  class HelloRequest
    string :greeting, number: 1
  end

  class HelloResponse
    string :reply, number: 1
  end
end

include Cameo

# HelloService is a module the Go program gives the script. say_hello
# sends the request and returns a promise at once; the Go program settles
# it with the HelloResponse when the answer is in. Every call is on its
# way before the first await returns.
def greet_all(names)
  calls = names.map { |name| HelloService.say_hello(Hello::HelloRequest.new(greeting: name)) }
  calls.map { |call| (await call).reply }
rescue HelloService::Failed => e
  ["the service failed: #{e.message}"]
end
```

### How the classes match the service {#grpc-match}

The service's messages are `hello.HelloRequest`, with the string `greeting` as
field 1, and `hello.HelloResponse`, with the string `reply` as field 1. Two
things make the script's classes the same messages:

-   A class in the module `Hello` is a message of the package `hello`: the
    package is the enclosing modules, in lower case.
-   `number:` sets a field's number. Without it cameo derives the number from
    the field's name, which suits messages only cameo reads.

`cameo proto` prints the `.proto` text of a script's classes, to compare with
the service's or to hand to another language:

``` sh
cameo proto examples/grpc/hello.cameo
```

``` proto
syntax = "proto3";

package hello;

message HelloRequest {
  optional string greeting = 1;
}

message HelloResponse {
  optional string reply = 1;
}
```

### The Go program {#grpc-go}

cameo does no I/O, so the Go program gives the script a module,
`HelloService`. Its `say_hello` gets the request as a `proto.Message`, sends
it on a goroutine, and returns a promise at once. The answer is read into a
message of the script's `Hello::HelloResponse`, which settles the promise.

``` {.go file="examples/grpc/main.go" data-title="main.go"}
// Command grpc calls a gRPC service from a cameo script. A class of a
// script is a protobuf message, so the request the script builds goes on
// the wire as it is, and the response comes back as an instance of a
// class of the script: no generated code and no conversion.
//
// The service is hello.HelloService of grpcb.in, a public test server
// that anyone may call:
//
//  go run . Ada Grace              # in examples/grpc
//  go run . -addr localhost:9000 -plaintext Ada
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
    "github.com/cameo-lang/cameo/src/cameoio"
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
    prog, err := cameo.LoadFromString("hello.cameo", script, cameoio.Streams(stdout, stderr))
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
```

The example is a Go module of its own, so cameo itself does not depend on
gRPC. [Embedding in Go](#go-proto) has more on passing messages both ways, and
[Native code](#native-classes) shows the other direction: a class backed by a
message Go code generated.
