## async, await and promises {#ch-async}

::: source
from `doc/gh_pages/async.html`
:::

A cameo script hands a method call to a worker with `async` and collects its
result with `await`. What `async` returns is a *promise*: a
name for a result to come, which a worker, the script itself or the Go host
settles. This page explains the model, shows it on one core, walks through the
examples in
[examples/async](https://github.com/cameo-lang/cameo/tree/main/examples/async),
and shows the same scripts running on Ruby.

1.  [How it works](#async-model)
2.  [When it runs on one core](#async-one-core)
3.  [When it runs on many cores](#async-cores)
4.  [Many workflows, I/O in the host](#async-io)
5.  [The examples](#async-examples)
6.  [The same script on Ruby](#async-ruby)
7.  [From a Go program: concurrency](#go-concurrency)
8.  [From a Go program: promises for I/O in the host](#go-promises)

### How it works {#async-model}

Concurrency comes from the `Cameo` module, which brings `async`, `await`,
`promise`, `resolve` and `fail`. The class that runs the work includes
`Cameo::Runner`, the same module under the name a Ruby program includes too;
see [below](#async-ruby). A script may instead include `Cameo` at the top
level, which brings the module to every method, but only cameo has that;
`Cameo::Runner` at the top level is refused on both. `Cameo.async` and
`Cameo.await` need no include.

`async receiver, :method, args…`
:   queues one call of a method on a receiver, over copies of the receiver and
    the arguments, and returns its promise at once. The call runs on a worker:
    an interpreter of its own, on a goroutine Go schedules over the cores.
    Nothing the call does reaches the caller's values, and its result comes
    back as a copy. A Proc is passed with the values it captured; a class
    cannot be passed, since it cannot be copied.

`await promise`
:   waits for the promise and returns its result, or raises the exception the
    call raised, with the same class and message. Awaiting a promise twice
    gives the same result. `await [a, b]` waits for each and returns the
    results in order; when some failed it waits for all, then raises the first
    failure in the list's order. `await :all` waits for every call the current
    method started and has not awaited, blocks included, in start order.
    `await receiver, :method, args…` starts a call and waits for it in one
    step.

`promise`, `resolve promise, value`, `fail promise, exception`
:   make a pending promise, and settle it with a value or an exception, once;
    settling it again raises. A promise is a value like any other, so it
    travels to a worker, which may settle it, and to the Go host, which settles the
    promises its methods return when their I/O is done; see [promises in the
    host](#go-promises). `fail` with anything but a promise first raises, as
    Ruby's `fail` does.

The calls `async` queues are taken by *runners*, up to one per core, started
as calls arrive and ended when the queue is empty. Two rules keep every call
able to run: a script about to wait for a promise while calls are queued
starts another runner, and `await` on a call nobody has taken runs that call
itself, on the goroutine that awaits it. A run ends when the script's top
level has and every call it started has; a call that failed and was never
awaited is reported on standard error.

### When it runs on one core {#async-one-core}

With one core, `async` cannot make calls that compute run faster: one of them
has the core at a time. What it buys is overlap with waiting. The timeline
below is a script that asks the host for exchange rates with `Web.fetch`,
which returns a promise and starts the request on a goroutine, and queues two
calls of its own, `parse` and `price`.

<figure>
<img src="../gh_pages/img/async-single-core.svg"
alt="A timeline with four lanes: the script, a runner, the host and the core. The script starts a fetch and queues two calls, runs parse itself when it awaits it, then waits for the rates while a runner takes price; the host resolves the promise when the response arrives, which wakes the script to finish." />
<figcaption>One core: the script waits for the network while a runner
computes. The bottom lane shows who has the core: one at a time. (<a
href="https://cameo-lang.github.io/cameo/img/async-single-core.excalidraw">Excalidraw
source</a>)</figcaption>
</figure>

1.  The script calls `Web.fetch`, which returns at once, and queues `parse`
    and `price` with `async`. The request is on its way; no core is needed for
    that.
2.  The script awaits `parse`. Nobody has taken it, so the script runs it
    itself rather than waiting for a runner.
3.  The script awaits the rates. It has nothing to do, so the core is free,
    and a runner takes `price`, the next call in the queue.
4.  The response arrives. The host settles the promise with `Resolve`, which
    wakes the script.
5.  The script awaits `price`, which is done, and prints the bill.

### When it runs on many cores {#async-cores}

With more cores, calls that compute run at the same time. Each queued call is
taken by a runner of its own, up to one per core, and Go spreads the runners
over the cores. [The Fibonacci fan-out](#ruby-async) queues four calls of
`fib(29)` and awaits the list of their promises:

<figure>
<img src="../gh_pages/img/async-four-cores.svg"
alt="A timeline with a script lane and four core lanes. The script queues four calls and waits; each core runs one call on a runner for the same stretch; the script then prints the results." />
<figcaption>Four cores: fib(29) four times, one call per core. (<a
href="https://cameo-lang.github.io/cameo/img/async-four-cores.excalidraw">Excalidraw
source</a>)</figcaption>
</figure>

On a four-core machine the four calls take 0.8 seconds where running them one
after another takes 1.4: under a 2× gain, not 4×, since the runners share the
memory system and the collector. The same script on one core takes as long as
the sequential one, as the [timeline above](#async-one-core) shows: there,
`async` buys overlap with waiting, not more computing.

### Many workflows, I/O in the host {#async-io}

The other shape of work is many workflows that each wait on the outside: fetch
a page, then measure it. Waiting costs no core, so one core runs as many
workflows at once as there are requests in flight. The host does the I/O: its
`Web.fetch` starts the request on a goroutine of its own and returns a
promise, and when the response is in, the host settles the promise *from
outside the script* with `Resolve` or `Fail`, which wakes the workflow that
awaits it.
[examples/webfetch](https://github.com/cameo-lang/cameo/tree/main/examples/webfetch)
is that program; [promises in the host](#go-promises) shows its Go side.

``` {.ruby .cameo}
include Cameo

class Flow
  # One workflow: fetch the page, then measure it. await parks this
  # workflow until the host settles the promise; the others run meanwhile.
  def run(url)
    page = await Web.fetch(url)
    "#{url}: #{page.size} bytes, #{page.lines.size} lines"
  rescue Web::FetchError => e
    "#{url}: #{e.message}"
  end
end

def fetch_all(urls)
  flow = Flow.new
  promises = urls.map { |url| async flow, :run, url }   # every workflow starts now
  await promises                                        # their lines, in order
end
```

<figure>
<img src="../gh_pages/img/async-io.svg"
alt="A timeline with a script lane, three workflow lanes, a host lane and a core lane. Each workflow fetches, then waits while the host has three requests on the wire; the host resolves all three, each workflow measures its page, and the core is busy only for the short starts and ends." />
<figcaption>One core, I/O in the host: every workflow's request is in flight
at once. (<a
href="https://cameo-lang.github.io/cameo/img/async-io.excalidraw">Excalidraw
source</a>)</figcaption>
</figure>

Three workflows start, each calls `Web.fetch` and awaits the promise; while
all three wait, the core is idle and the three requests are on the wire
together. The host's goroutines settle the three promises as the responses
arrive, each workflow measures its page, and `await promises` at the top
returns the three lines. The test of `examples/webfetch` holds the program to
this: its server answers a request only once all of them have arrived, so the
run finishes only if every workflow's fetch was in flight at once.

### The examples {#async-examples}

Each script below is a file in `examples/async`, with the output it prints.
The scripts start with `require "cameo/runner"`, which cameo builds in, and
the class that runs the work includes `Cameo::Runner`, so that the same file
runs on Ruby; see [the next section](#async-ruby).

#### one call {#async-01_one_call}

async starts one method call on a worker and returns its promise
at once; await waits for the call and returns its result. A class includes
Cameo::Runner to run its work so.

``` {.ruby .cameo}
require "cameo/runner"

class Calculator
  include Cameo::Runner

  def square(n)
    n * n
  end

  def demo
    job = async self, :square, 12
    puts "the promise: #{job}"
    puts "its result: #{await job}"

    # async and await in one step: start the call and wait for it.
    puts "in one step: #{await self, :square, 7}"
  end
end

Calculator.new.demo
```

``` output
the promise: 1
its result: 144
in one step: 49
```

#### fan out {#async-02_fan_out}

Fan out: start every call before awaiting any, then await the list of
promises, which returns the results in the order of the list.

``` {.ruby .cameo}
require "cameo/runner"

class Words
  include Cameo::Runner

  def count(text)
    text.split.size
  end

  def count_all(pages)
    promises = pages.map { |page| async self, :count, page }
    await promises
  end
end

pages = [
  "the quick brown fox",
  "jumps over",
  "the lazy dog and keeps on running"
]
counts = Words.new.count_all(pages)
puts counts.inspect
puts "#{counts.sum} words on #{pages.size} pages"
```

``` output
[4, 2, 7]
13 words on 3 pages
```

#### await all {#async-03_await_all}

await :all waits for every call the current method started and has not
awaited, blocks included, and returns their results in start order.

``` {.ruby .cameo}
require "cameo/runner"

class Squares
  include Cameo::Runner

  def of(n)
    n * n
  end

  def table(limit)
    (1..limit).each { |n| async self, :of, n }
    await :all
  end

  # The calls a method started belong to that method: here only the two
  # started in pair are collected, not table's.
  def pair
    async self, :of, 10
    async self, :of, 20
    await :all
  end
end

squares = Squares.new
puts squares.table(5).inspect
puts squares.pair.inspect
```

``` output
[1, 4, 9, 16, 25]
[100, 400]
```

#### failures {#async-04_failures}

A call that raises raises again where it is awaited, with the same class and
message. Awaiting a list waits for every call, then raises the first failure
in the list's order.

``` {.ruby .cameo}
require "cameo/runner"

class Inventory
  include Cameo::Runner

  def reserve(item, qty)
    raise ArgumentError, "no #{item} left" if qty > 3

    "#{qty} x #{item}"
  end

  def demo
    begin
      await self, :reserve, "lamps", 5
    rescue ArgumentError => e
      puts "raised: #{e.message}"
    end

    orders = [["books", 1], ["chairs", 9], ["pens", 2], ["desks", 4]]
    promises = orders.map { |item, qty| async self, :reserve, item, qty }
    begin
      await promises
    rescue ArgumentError => e
      puts "the list raised: #{e.message}"
    end
    # The calls that worked still have their results.
    puts await(promises[0]), await(promises[2])
  end
end

Inventory.new.demo
```

``` output
raised: no lamps left
the list raised: no chairs left
1 x books
2 x pens
```

#### promises {#async-05_promises}

A promise made by hand: promise makes one, and resolve or fail settles it,
once. A promise is a value like any other, so it travels to a worker,
and the worker settles it while the caller awaits it.

``` {.ruby .cameo}
require "cameo/runner"

class Courier
  include Cameo::Runner

  def deliver(promise, parcel)
    resolve(promise, "#{parcel}, delivered")
  end

  def lose(promise, parcel)
    fail(promise, KeyError, "#{parcel} is lost")
  end

  def demo
    parcel = promise
    async self, :deliver, parcel, "books"
    puts await(parcel)

    missing = promise
    async self, :lose, missing, "a lamp"
    begin
      await missing
    rescue KeyError => e
      puts "failed: #{e.message}"
    end

    begin
      resolve parcel, "again"
    rescue ArgumentError => e
      puts e.message
    end

    # fail with anything but a promise first raises, as Ruby's fail does.
    begin
      fail "not a promise"
    rescue RuntimeError => e
      puts "raised: #{e.message}"
    end
  end
end

Courier.new.demo
```

``` output
books, delivered
failed: a lamp is lost
promise 1 is settled already
raised: not a promise
```

#### workflow {#async-06_workflow}

A workflow of steps that depend on each other. Checking the order and pricing
its lines run at once; shipping waits for both, and its label is made while
the parcel is weighed.

``` {.ruby .cameo}
require "cameo/runner"

class Shop
  include Cameo::Runner

  def check(order)
    order["lines"].all? { |line| line["qty"].positive? }
  end

  def price(line)
    line["qty"] * line["cents"]
  end

  def weigh(order)
    order["lines"].sum { |line| line["qty"] * line["grams"] }
  end

  def label(order)
    "#{order["id"]} to #{order["to"]}"
  end

  def ship(order)
    checked = async self, :check, order
    prices = order["lines"].map { |line| async self, :price, line }
    raise ArgumentError, "order #{order["id"]} has an empty line" unless await checked

    total = await(prices).sum
    weight = async self, :weigh, order
    label = async self, :label, order
    "#{await label}: #{total} cents, #{await weight} g"
  end
end

order = {
  "id" => "A-17", "to" => "Berlin",
  "lines" => [
    { "qty" => 2, "cents" => 1250, "grams" => 300 },
    { "qty" => 1, "cents" => 4999, "grams" => 1200 }
  ]
}
puts Shop.new.ship(order)
```

``` output
A-17 to Berlin: 7499 cents, 1800 g
```

#### copies {#async-07_copies}

A call that async starts gets copies of its receiver and its arguments, and
its result comes back as a copy: nothing a worker does reaches the caller's
values. A Proc crosses too, with the values it captured. A class cannot be
copied, so it cannot be passed.

``` {.ruby .cameo}
require "cameo/runner"

class Editor
  include Cameo::Runner

  def append(list, item)
    list + [item]
  end

  def each_changed(list, change)
    list.map { |item| change.call(item) }
  end

  def demo
    list = [1, 2]
    longer = await self, :append, list, 3
    puts "the worker's: #{longer.inspect}, the caller's: #{list.inspect}"

    factor = 10
    scaled = await self, :each_changed, list, ->(item) { item * factor }
    puts "changed by a proc: #{scaled.inspect}"

    begin
      async self, :append, list, String
    rescue TypeError => e
      puts e.message
    end
  end
end

Editor.new.demo
```

``` output
the worker's: [1, 2, 3], the caller's: [1, 2]
changed by a proc: [10, 20]
a Class cannot be passed to async
```

#### top level {#async-08_top_level}

Cameo::Runner belongs in the class that runs the work. The top level refuses
it, on cameo and on Ruby alike; a script that wants async at the top level
includes Cameo, which only cameo has.

``` {.ruby .cameo}
require "cameo/runner"

begin
  include Cameo::Runner
rescue StandardError => e
  puts "refused: #{e.message}"
end
```

``` output
refused: include Cameo::Runner belongs in the class that runs the work; at the top level include Cameo
```

### The same script on Ruby {#async-ruby}

[ruby/lib/cameo/runner.rb](https://github.com/cameo-lang/cameo/blob/main/ruby/lib/cameo/runner.rb)
gives CRuby the `Cameo::Runner` module: `async`, `await`, `promise`, `resolve`
and `fail` with cameo's results, promise ids and error messages. A script
whose classes include it runs unchanged on both:

``` sh
$ cameo examples/async/02_fan_out.rb
[4, 2, 7]
13 words on 3 pages
$ ruby -I ruby/lib examples/async/02_fan_out.rb
[4, 2, 7]
13 words on 3 pages
```

On Ruby, `async` copies the receiver and the arguments with `Marshal`, as
cameo copies them, and runs the call on a thread of its own; a Method, a class
or a module cannot be copied, as in cameo. A Proc is passed as it is: cameo
sends its code and the values it captured, and Ruby has no copy of one, so on
Ruby a proc that changes a variable around it changes it for the caller too.
The result comes back as a copy, and a failure raises again where the call is
awaited. `await :all` collects the calls the current method started, which the
library tells from the caller's location.

What differs is the cores. CRuby's global VM lock runs one thread's Ruby code
at a time, so calls that compute take turns on one core, as in the timeline
above, where cameo spreads them over the cores. A call that waits, for the
network or a sleep, lets the others run on both. The library is for running a
cameo script where cameo is not embedded; it does not make Ruby parallel.

### From a Go program: concurrency {#go-concurrency}

Inside a script that includes `Cameo`, `async` runs a method call on another
goroutine, over copies of its receiver and arguments; see [async, await and
promises](#ruby-async). Every worker reaches the host's modules, so a host
method may run on several goroutines at once and must be safe for that.
`RunContext` stops the workers with the script.

``` {.cameo .hosted data-title="fetch.cameo"}
include Cameo

class Fetcher
  string :base

  def get(path)
    Http.get(base + path)
  end
end

f = Fetcher.new(base: "https://example.com")
jobs = ["/a", "/b", "/c"].map { |path| async f, :get, path }
p jobs.map { |job| await job }
```

``` go
package main

import (
    "fmt"
    "log"
    "os"
    "sync/atomic"

    "github.com/cameo-lang/cameo/src/cameo"
    "github.com/cameo-lang/cameo/src/cameoio"
)

func main() {
    prog, err := cameo.Load("fetch.cameo", cameoio.DefaultEnv())
    if err != nil {
        log.Fatal(err)
    }
    var calls atomic.Int64
    prog.Module("Http").Def("get", func(args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
        calls.Add(1) // runs on each worker's goroutine
        return "body of " + args[0].(string), nil
    })
    if err := prog.Run(); err != nil {
        log.Fatal(err)
    }
    fmt.Println(calls.Load(), "calls")
}
```

``` output
["body of https://example.com/a", "body of https://example.com/b", "body of https://example.com/c"]
3 calls
```

### From a Go program: promises for I/O in the host {#go-promises}

cameo does no I/O. A host method that does some, such as fetching a web page,
can return a *promise* instead of blocking: it makes one with
`prog.Promise()`, starts the work on a goroutine, and returns the promise at
once. The script holds the promise and awaits it; the goroutine
settles it with `Resolve`, with a Go value that converts as a host method's
result does, or with `Fail`, with an error that `await` raises. The script
below runs one workflow per page, each awaiting its own fetch, so every
request is in flight while the workflows wait; [the async guide](#async-io)
shows the timeline. `Resolve` and `Fail` may be called from any goroutine,
once; a second call returns `cameo.ErrSettled`. This is
[examples/webfetch](https://github.com/cameo-lang/cameo/tree/main/examples/webfetch):

``` {.cameo .hosted file="examples/webfetch/webfetch.cameo" data-title="webfetch.cameo"}
include Cameo

class Flow
  # One workflow: fetch the page, then measure it. await parks this
  # workflow until the host settles the promise; the others run meanwhile.
  def run(url)
    page = await Web.fetch(url)
    "#{url}: #{page.size} bytes, #{page.lines.size} lines"
  rescue Web::FetchError => e
    "#{url}: #{e.message}"
  end
end

def fetch_all(urls)
  flow = Flow.new
  promises = urls.map { |url| async flow, :run, url }   # every workflow starts now
  await promises                                        # their lines, in order
end
```

``` go
// Command webfetch fetches web pages from a cameo script, one workflow per
// page. cameo does no I/O, so the host gives the script a Web module whose
// fetch starts the request on a goroutine and returns a promise at once; the
// host settles the promise from outside the script when the response is
// in. Each workflow awaits its own fetch, so every request is in flight
// while the script waits, and await returns each page or raises
// Web::FetchError.
//
// The script, webfetch.cameo, is built into the program, and cameo loads it
// through an Env of the program's own files.
//
//  go run ./examples/webfetch https://example.com https://example.org/missing
package main

import (
    "embed"
    "fmt"
    "io"
    "net/http"
    "os"

    "github.com/cameo-lang/cameo/src/cameo"
    "github.com/cameo-lang/cameo/src/cameoio"
    "github.com/spf13/afero"
)

//go:embed webfetch.cameo
var scripts embed.FS

// fetchAll runs the script's fetch_all over urls with client.
func fetchAll(client *http.Client, urls []string) ([]any, error) {
    env := cameoio.New(afero.FromIOFS{FS: scripts}, ".", nil, os.Stdout, os.Stderr)
    prog, err := cameo.Load("webfetch.cameo", env)
    if err != nil {
        return nil, err
    }
    web := prog.Module("Web")
    fetchError := web.Exception("FetchError")
    web.Def("fetch", func(args []any, kwargs map[string]any, block *cameo.Block) (any, error) {
        url, ok := args[0].(string)
        if !ok {
            return nil, fmt.Errorf("Web.fetch takes a URL, not %v", args[0])
        }
        promise := prog.Promise()
        go func() {
            resp, err := client.Get(url)
            if err != nil {
                promise.Fail(fetchError.New("%v", err))
                return
            }
            defer resp.Body.Close()
            if resp.StatusCode != http.StatusOK {
                promise.Fail(fetchError.New("%s", resp.Status))
                return
            }
            body, err := io.ReadAll(resp.Body)
            if err != nil {
                promise.Fail(fetchError.New("%v", err))
                return
            }
            promise.Resolve(string(body))
        }()
        return promise, nil
    })
    lines, err := prog.Call(nil, "fetch_all", urls)
    if err != nil {
        return nil, err
    }
    return lines.([]any), nil
}

func main() {
    lines, err := fetchAll(http.DefaultClient, os.Args[1:])
    if err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
    for _, line := range lines {
        fmt.Println(line)
    }
}
```

``` sh
$ go run ./examples/webfetch \
    https://raw.githubusercontent.com/cameo-lang/cameo/main/README.md \
    https://raw.githubusercontent.com/cameo-lang/cameo/main/LICENSE \
    https://raw.githubusercontent.com/cameo-lang/cameo/main/MISSING.md
https://raw.githubusercontent.com/cameo-lang/cameo/main/README.md: 8781 bytes, 177 lines
https://raw.githubusercontent.com/cameo-lang/cameo/main/LICENSE: 1465 bytes, 28 lines
https://raw.githubusercontent.com/cameo-lang/cameo/main/MISSING.md: 404 Not Found
```

The host API makes no promise that one `Program` can be used from several
goroutines at once. Give each goroutine of the host its own `Program`; loading
one takes microseconds.
