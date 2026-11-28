## Extending cameo with Go code {#ch-native}

::: source
from `doc/gh_pages/native.html`
:::

This chapter belongs with [loading and running a script](#go-run): once your
program runs a script, it can give the script Go code to call. Find what you
want to do, then do what the section says:

1.  If you want a script to call some Go functions, give the program
    [a module of Go functions](#native-host). Only that program sees it.
2.  If you want a script to use a class whose instances are your Go
    protobuf messages, [register the class](#native-classes). Every program
    in your process sees it.
3.  If you want a class to be part of cameo itself, for every host, write
    [a builtin class](#native-stdlib) in cameo's standard library.

### To give a script Go functions: a module {#native-host}

Between `Load` and `Run`, a host defines modules for the script:

-   `prog.Module(name)` defines the module `name` as a constant at the
    script's top level, as `Math` is, and returns a `*cameo.Module`. A script
    cannot redefine or reopen it.
-   `mod.Def(name, fn)` defines the method the script calls as
    `Name.name(args)`. `fn` is a `cameo.Func`,
    `func(args []any, kwargs map[string]any, block *cameo.Block) (any, error)`:
    it gets the positional and keyword arguments as Go values, as `ToGo`
    converts them, except that a Symbol is a `string` and a Hash with String
    or Symbol keys is a `map[string]any`, and the block of the call as
    `block`, nil without one. A Proc among the arguments is a `*cameo.Block`
    too; `block.Call(args...)` runs either with Go values and returns the
    result as `Call` does. What `fn` returns converts as `Call`'s arguments
    do, a `*cameo.Block` back into its Proc.
-   `mod.Exception(name)` defines the exception class `Name::name`, a
    `StandardError`. Its `New(format, args...)` returns the error that raises
    it when `fn` returns it. Any other error raises `RuntimeError` with the
    error's text.

``` {.cameo .hosted data-title="text.cameo"}
puts Text.shout("hello", times: 2)
begin
  Text.shout(42)
rescue Text::Invalid => e
  puts "rescued: #{e.message}"
end
```

``` go
package main

import (
    "log"
    "os"
    "strings"

    "github.com/cameo-lang/cameo/src/cameo"
    "github.com/cameo-lang/cameo/src/cameoio"
)

func main() {
    prog, err := cameo.Load("text.cameo", cameoio.DefaultEnv())
    if err != nil {
        log.Fatal(err)
    }
    text := prog.Module("Text")
    invalid := text.Exception("Invalid")
    text.Def("shout", func(args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
        s, ok := args[0].(string)
        if !ok {
            return nil, invalid.New("shout takes a String, not %T", args[0])
        }
        times := int64(1)
        if n, ok := kwargs["times"].(int64); ok {
            times = n
        }
        return strings.Repeat(strings.ToUpper(s)+"!", int(times)), nil
    })
    if err := prog.Run(); err != nil {
        log.Fatal(err)
    }
}
```

``` output
HELLO!HELLO!
rescued: shout takes a String, not int64
```

Workers that `async` starts call host methods on goroutines of their own, so a
host method may run on several at once. One that blocks keeps the script from
stopping until it returns: watch the context the host passed to `RunContext`.

#### A worked example: a module of tools

[ark](https://github.com/radiospiel/ark) runs scripts that call tools on
cameo. Its host gives every script a module `Spicy`, in the shape a module of
this kind takes:

-   `Spicy.tools` alone returns the list of tools;
    `Spicy.tools("search", { "q" => "x" })` or `Spicy.tools("search", q: "x")`
    calls one, merging the Hash and the keywords into the tool's JSON input.
-   Every failure returns `toolError.New(…)`, so a script rescues
    `Spicy::ToolError`.
-   The host runs the script with `RunContext` and passes the same
    `context.Context` to the tool call, so a cancelled run reaches a tool that
    is still working.

### To give a script a class of Go messages: register it {#native-classes}

A host registers a class of its own with `cameo.Register`. The class's data
layout is a protobuf message, generated with `protoc-gen-go`, and its methods
are Go functions.

#### 1. The message

Write the fields as a protobuf message:

``` proto
syntax = "proto3";

package example.geo;

option go_package = "example.com/geo/geopb";

message Point {
  int64 x = 1;
  int64 y = 2;
}
```

#### 2. The Go type

Generate the Go type with `protoc` and `protoc-gen-go`:

``` sh
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
protoc --go_out=. --go_opt=module=example.com/geo geo.proto
```

This writes `geopb/geo.pb.go`, with the type `geopb.Point`.

#### 3. Register the class

Register the class in an `init` function, then load scripts as usual. Here the
script uses `Geo::Point`, and Go creates one through `Call`:

``` {.cameo .hosted data-title="geo.cameo"}
pt = Geo::Point.new(x: 3, y: 4)
p pt.norm2
p pt.moved(1).x, pt.x
p Geo::Point.origin.with(y: 2)
p Geo::Point.new.x
```

``` go
package main

import (
    "fmt"
    "log"
    "os"

    "example.com/geo/geopb"
    "github.com/cameo-lang/cameo/src/cameo"
    "github.com/cameo-lang/cameo/src/cameoio"
)

func init() {
    cameo.Register("Geo::Point", &geopb.Point{}).
        DefineMethod("norm2", func(p *geopb.Point, args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
            return p.X*p.X + p.Y*p.Y, nil
        }).
        DefineMethod("moved", func(p *geopb.Point, args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
            p.X += args[0].(int64) // p is a copy; the changed copy is the result
            return p, nil
        }).
        DefineClassMethod("origin", func(args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
            return &geopb.Point{}, nil
        })
}


func main() {
    prog, err := cameo.Load("geo.cameo", cameoio.DefaultEnv())
    if err != nil {
        log.Fatal(err)
    }
    if err := prog.Run(); err != nil {
        log.Fatal(err)
    }
    pt, err := prog.Call("Geo::Point", "new", map[string]any{"x": 5, "y": 12})
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("%T x=%d\n", pt, pt.(*geopb.Point).X) // a registered class leaves cameo as its generated type
}
```

``` output
25
4
3
Geo::Point(x: 0, y: 2)
0
*geopb.Point x=5
```

#### The rules

-   **The class's fields are the message's.** Every proto3 scalar type is
    supported. Repeated fields are lists, and maps take a scalar key. Enums
    must be numbered from 0 without a gap; their values become symbols
    (`COLOR_RED` is `:red`). Do not give the value 0 a meaning: name it
    `COLOR_UNSPECIFIED`, for example. An unset field reads as 0, so a
    meaningful 0 cannot be told from a field that nobody set. A field may hold a message whose class was
    registered earlier, or the message itself. `oneof` is not supported.
-   **Unset fields read as they do in Go.** A field without presence reads as
    its zero value, so `Geo::Point.new.x` is `0`. A field with presence
    (`optional`, or a message) reads as `nil`.
-   **Methods get copies.** A method gets `self` as the generated type: a copy
    it may change without changing the instance. A returned message of a
    registered type becomes an instance, so a method returns a changed copy,
    as `moved` does above. Arguments, results and errors convert as they do
    for a host module's methods.
-   **Instances leave cameo as the generated type.** `Call` and `ToGo` return
    a `*geopb.Point`, and a `*geopb.Point` passed in becomes an instance.
    `async` passes instances to workers like any other value.
-   **Names are closed.** A qualified name creates the modules it needs. A
    registered class is a constant in every program in the process, and no
    script can redefine it or define anything inside it.
-   **Register before the first `Load`**, from an `init` function. `Register`
    panics after that, or when the name or the message cannot be used. Both
    are mistakes in the host's code.

#### Instances of a class the script declares

A host method can also return an instance of a class that the script
declares itself, not the host. The host does not have a Go type for such a
class. It asks the program for the class's protobuf descriptor with
`prog.Descriptor(name)`, builds a `dynamicpb` message from that descriptor,
and returns the message. cameo sees that the message has the descriptor of a
loaded class, so the script gets an instance of that class.

In the example below, the script declares `User`, and the host's `Users.find`
returns a `User`. The script calls `greeting` on it, as on any other
`User`.

``` {.cameo .hosted data-title="users.cameo"}
class User
  string :name
  int :age

  def greeting
    "#{name} is #{age}"
  end
end

user = Users.find("ada")
puts user.greeting
p user.with(age: user.age + 1)
```

``` go
package main

import (
    "log"
    "os"

    "google.golang.org/protobuf/reflect/protoreflect"
    "google.golang.org/protobuf/types/dynamicpb"

    "github.com/cameo-lang/cameo/src/cameo"
    "github.com/cameo-lang/cameo/src/cameoio"
)

func main() {
    prog, err := cameo.Load("users.cameo", cameoio.DefaultEnv())
    if err != nil {
        log.Fatal(err)
    }
    prog.Module("Users").Def("find", func(args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
        // The script declares User; Go builds one from its descriptor.
        desc, err := prog.Descriptor("User")
        if err != nil {
            return nil, err
        }
        user := dynamicpb.NewMessage(desc)
        user.Set(desc.Fields().ByName("name"), protoreflect.ValueOfString(args[0].(string)))
        user.Set(desc.Fields().ByName("age"), protoreflect.ValueOfInt64(36))
        return user, nil
    })
    if err := prog.Run(); err != nil {
        log.Fatal(err)
    }
}
```

``` output
ada is 36
User(name: "ada", age: 37)
```

### To add a class to cameo itself: a builtin class {#native-stdlib}

cameo's builtin classes live in `src/stdlib`, one file per area. A class is an
`*object.Class` from `object.NewClass(name, super)`:

-   `c.Define(name, arity, fn)` adds an instance method and
    `c.DefineSingleton(name, arity, fn)` a class method, where `fn` is an
    `object.NativeFn`:
    `func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error)`.
    An arity of `-1` takes any number of arguments. `ctx` raises exceptions
    (`ctx.Raise`), calls methods and blocks, and renders values.
-   An instance is an `object.Instance` with the class and its `Fields`, as an
    `Encoding` holds its name.
-   An `install…` function defines the methods and binds the constant, and
    `stdlib.Install` calls it.
-   `puts` and `p` call the class's own native `to_s` and `inspect`.

A library that a script loads with `require`, as Ruby's standard library is,
goes in a Go package of its own under `src/lib`, named for its `require` path
and listed in `lib.Libraries`; [doc/stdlib.md](#ch-reference-stdlib) tracks
them.

This adds `Money`, an amount in cents, in `src/stdlib/money.go`:

``` go
package stdlib

import (
    "fmt"

    "github.com/cameo-lang/cameo/src/object"
    "github.com/cameo-lang/cameo/src/str"
)

// MoneyClass is Money: an amount in cents, held as its instance's one field.
var MoneyClass = object.NewClass("Money", object.ObjectClass)

// installMoney defines Money.new(cents), Money#cents, Money#+, Money#to_s
// and Money#inspect.
func installMoney() {
    c := MoneyClass
    object.ObjectClass.Consts["Money"] = object.ClassValue(c)
    money := func(cents int64) object.Value {
        return object.InstanceValue(&object.Instance{Cls: c, Fields: []object.Value{object.Int(cents)}})
    }
    cents := func(v object.Value) int64 { return v.Instance().Fields[0].Int() }
    dollars := func(v object.Value) string {
        n := cents(v)
        return fmt.Sprintf("$%d.%02d", n/100, n%100)
    }

    c.DefineSingleton("new", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
        n, err := WantInt(ctx, args[0], "")
        if err != nil {
            return object.Value{}, err
        }
        return money(n), nil
    })
    def(c, "cents", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
        return object.Int(cents(self)), nil
    })
    def(c, "+", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
        other, ok := args[0].AsInstance()
        if !ok || other.Cls != c {
            return object.Value{}, ctx.Raise(object.TypeError, "Money can't be added to %s", KindName(args[0]))
        }
        return money(cents(self) + cents(args[0])), nil
    })
    // puts and p call a builtin class's own to_s and inspect.
    def(c, "to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
        return object.NewString(dollars(self), str.UTF8), nil
    })
    def(c, "inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
        return object.NewString("#<Money "+dollars(self)+">", str.UTF8), nil
    })
}
```

and installs it:

``` diff
diff --git a/src/stdlib/stdlib.go b/src/stdlib/stdlib.go
--- a/src/stdlib/stdlib.go
+++ b/src/stdlib/stdlib.go
@@ -41,6 +41,7 @@ func Install() {
    installPaths()
    installVersion()
    installIO()
+   installMoney()
    // After Enumerable and Enumerator: it wraps Enumerable's methods.
    installUserEnumerable()
 }
```

Built with these changes, cameo runs:

``` {.ruby .cameo .unbuilt}
total = Money.new(1250) + Money.new(199)
puts total
p total
p total.cents
p total == Money.new(1449)
begin
  total + 1
rescue TypeError => e
  puts e.message
end
```

``` output
$14.49
#<Money $14.49>
1449
true
Money can't be added to Integer
```

A builtin class's instances are not protobuf messages: they cannot be passed
to `async`, which raises `TypeError`, "a Money cannot be passed to async", and
they reach a host as an `*object.Instance`. A new builtin also comes with an
example in `examples/`, whose `.expected` output `go test` holds it to, and a
line in `doc/cameo-vs-ruby.md` if Ruby has the class too.
