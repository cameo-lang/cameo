# async and await

Use them where pieces of work do not depend on each other: one call per
item of a list, several requests to the host, a slow computation beside
other work. Do not use them for a single call or for steps that feed each
other; plain calls are simpler and as fast.

```ruby
include Cameo                       # once, at the top level

class Report
  string :name

  def score = (1..200_000).sum { |n| n % 7 } + name.length
end

reports = %w[north south east west].map { |n| Report.new(name: n) }

jobs = reports.map { |r| async r, :score }     # start every call first
scores = jobs.map { |job| await job }          # then collect
p reports.map(&:name).zip(scores).to_h

class Flaky
  def run = raise(ArgumentError, "bad input")
end

begin
  await async(Flaky.new, :run)                 # a worker's exception is
rescue ArgumentError => e                      # raised where it is awaited
  puts "failed: #{e.message}"
end
```

- `async receiver, :method, *args` starts the call on a goroutine and
  returns a promise. Values are immutable, so the call shares nothing
  with the caller and needs no lock.
- `await promise` returns the result or raises the call's exception.
  `await [a, b]` awaits several, and `await :all` everything the current
  scope started.
- Start all the calls before the first `await`. `await async(…)` in a
  loop runs them one after another.
- A function of the host that does I/O usually returns a promise:
  `page = await Web.fetch(url)`.
