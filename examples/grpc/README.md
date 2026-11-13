# grpc: a cameo script that calls a gRPC service

A class of a cameo script is a protobuf message. So a script can build the
request of a gRPC call and read its response as instances of its own
classes, and the Go program between them passes the messages on as they
are: no generated code and no conversion.

The service is `hello.HelloService` of [grpcb.in](https://grpcb.in), a
public test server (github.com/moul/grpcbin) that anyone may call without
a key.

```sh
cd examples/grpc
go run . Ada Grace                              # TLS, grpcb.in:9001
go run . -addr grpcb.in:9000 -plaintext Ada     # without TLS
```

```
hello Ada
hello Grace
```

| File | Holds |
|---|---|
| [hello.cameo](hello.cameo) | the messages as classes, and `greet_all`, which calls the service once per name and awaits the answers |
| [main.go](main.go) | gives the script the module `HelloService`, whose `say_hello` sends the request on a goroutine and returns a promise |
| [main_test.go](main_test.go) | the same against a service on this machine, so the test needs no network |

## How the classes match the service

The service's messages are `hello.HelloRequest`, with the string
`greeting` as field 1, and `hello.HelloResponse`, with the string `reply`
as field 1. In the script:

```ruby
module Hello
  class HelloRequest
    string :greeting, number: 1
  end

  class HelloResponse
    string :reply, number: 1
  end
end
```

A class in the module `Hello` is a message of the package `hello`, and
`number:` sets the field's number, which cameo otherwise derives from the
name. `cameo proto examples/grpc/hello.cameo` prints the `.proto` text of
the classes, to compare with the service's.

## Why it is a module of its own

This directory has its own `go.mod`, so that cameo's does not require
`google.golang.org/grpc`. `make tests` runs its tests from inside it.
