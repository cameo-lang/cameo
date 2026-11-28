## Protobuf all the way down {#ch-proto}

::: source
from `doc/gh_pages/go.html`
:::

cameo keeps every value as protobuf. An instance of a class is a protobuf
message, so it goes to Go, to a file or to a gRPC service as it is. The
state of a paused run is protobuf too. This chapter shows how a Go value
becomes a cameo value, and how a class becomes a message.

### Go values and cameo values {#go-values}

`prog.ToValue(v)` converts a Go value to a cameo value (an `object.Value`),
and `prog.ToGo(v)` converts back.

Each group of values has a purpose.

**Numbers and truth.** These are the plain values of a computation.

| Go value | cameo value | back in Go |
|---|---|---|
| `nil` | `nil` | `nil` |
| `bool` | `true`, `false` | `bool` |
| `int`, `int8` … `int64`, `uint8` … `uint32` | Integer | `int64` |
| `uint`, `uint64`, `*big.Int` | Integer | `int64` when it fits, else `*big.Int` |
| `float32`, `float64` | Float | `float64` |

**Text, bytes and time.** A Go `string` is text, a `[]byte` is binary data.

| Go value | cameo value | back in Go |
|---|---|---|
| `string` | String (UTF-8) | `string` |
| `[]byte` | String (binary) | `[]byte` |
| `time.Time` | Time | `time.Time` |

**Collections.** They carry many values at once. A Go value never becomes a
Set. To get a Set, the script makes one from an Array: `Set.new(list)`. A Set
that goes back to Go is a `[]any`.

| Go value | cameo value | back in Go |
|---|---|---|
| `[]any`, `[]string` | Array | `[]any` |
| — | Set | `[]any` |
| `map[string]any`, `map[any]any` | Hash | `map[any]any` |

**Messages.** An instance of a class is a protobuf message. This is how
structured data crosses between Go and a script. The next section has the
detail.

| Go value | cameo value | back in Go |
|---|---|---|
| `proto.Message` of a loaded class | an instance of that class | `proto.Message` (a `*dynamicpb.Message`) |

**Code and the rest.** A block goes to Go as a value that Go can call. A cameo
value that has no Go form stays a cameo value.

| Go value | cameo value | back in Go |
|---|---|---|
| `*cameo.Block` | Proc | `*cameo.Block`, whose `Call` runs it |
| `object.Value` | itself | — |
| — | Symbol, Range, Class and the rest | the `object.Value` itself |

Any other Go value is an error: "cannot convert struct {} to a cameo value". A
Hash's Symbol keys become strings in Go, as a Symbol does wherever a value
leaves cameo. A Hash that has no Go map form is an error: one with both `:a`
and `"a"` as keys, or with keys that are not comparable in Go, such as a Range
or a binary String.

### Protobuf messages, from the script to Go and back {#go-proto}

Every class a script defines is a protobuf message, with one field per
declared field. A field's number comes from a hash of its name, or from
`number:` in its declaration; see [Protobuf](#ruby-proto) on the Ruby page. A
class inside `module Geo` is the message `Point` in package `geo`.

-   `prog.Proto(name)` returns the `.proto` text of one class.
-   `prog.ProtoFiles()` returns the `.proto` files of every loaded class, one
    per package and keyed by file name, with the value envelope in
    `cameo/value.proto`. Compile them with `protoc` to get Go types with the
    same wire form.
-   `prog.Descriptor(name)` returns the class's
    `protoreflect.MessageDescriptor`, from which `dynamicpb` builds a message
    without generated code.
-   `prog.Encode(v)` writes any value, not only an instance, in the
    `cameo.Value` envelope; `prog.Decode` reads it back.
-   `protogen.Merge(registered, next)` (package
    `github.com/cameo-lang/cameo/src/protogen`) merges two definitions of one
    message, as two programs that share a store have: the fields both have
    stay as registered, and the fields only `next` has are added, numbered
    from their names. The [design document](#ch-design-language) says what it
    accepts.

``` {.cameo .hosted data-title="geo.cameo"}
module Geo
  class Point
    int :x, :y

    def norm2
      x * x + y * y
    end
  end
end
```

``` go
package main

import (
    "fmt"
    "log"
    "maps"
    "slices"

    "google.golang.org/protobuf/proto"
    "google.golang.org/protobuf/reflect/protoreflect"
    "google.golang.org/protobuf/types/dynamicpb"

    "github.com/cameo-lang/cameo/src/cameo"
    "github.com/cameo-lang/cameo/src/cameoio"
)

func check(err error) {
    if err != nil {
        log.Fatal(err)
    }
}

func main() {
    prog, err := cameo.Load("geo.cameo", cameoio.DefaultEnv())
    check(err)
    check(prog.Run())

    // The .proto text of one class, and the files of all of them.
    text, err := prog.Proto("Geo::Point")
    check(err)
    fmt.Print(text)
    files := prog.ProtoFiles()
    fmt.Println(slices.Sorted(maps.Keys(files)))

    // A message built in Go from the class's descriptor is an instance in
    // the script.
    desc, err := prog.Descriptor("Geo::Point")
    check(err)
    msg := dynamicpb.NewMessage(desc)
    msg.Set(desc.Fields().ByName("x"), protoreflect.ValueOfInt64(3))
    msg.Set(desc.Fields().ByName("y"), protoreflect.ValueOfInt64(4))
    n, err := prog.Call(msg, "norm2")
    check(err)
    fmt.Println(n)

    // An instance comes back as a proto.Message.
    pt, err := prog.Call("Geo::Point", "new", map[string]any{"x": 1, "y": 2})
    check(err)
    wire, err := proto.Marshal(pt.(proto.Message))
    check(err)
    fmt.Println(len(wire), "bytes")

    // Any value travels in the cameo.Value envelope.
    data, err := prog.Encode([]any{"a", int64(1), pt})
    check(err)
    back, err := prog.Decode(data)
    check(err)
    fmt.Println(len(back.([]any)), "elements")
}
```

``` output
package geo;

message Point {
  optional int64 x = 64843;
  optional int64 y = 64608;
}
[cameo/value.proto geo.proto]
25
8 bytes
3 elements
```

