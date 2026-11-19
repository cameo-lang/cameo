# Recursion: end in the call

A call that is the value of its method replaces its caller, so a method
that ends by calling itself runs as a loop and is not limited by the
stack. Ruby keeps a frame per call; cameo does not. Carry the result in
an argument instead of computing after the call returns.

```ruby
def sum_to(n, acc = 0)
  return acc if n == 0

  sum_to(n - 1, acc + n)          # a tail call: any depth
end

def sum_slow(n) = n == 0 ? 0 : n + sum_slow(n - 1)   # the + runs after: SystemStackError when deep

puts sum_to(1_000_000)
```

`cameo ir script.cameo` prints what the script compiles to; a tail call
is a `SEND` marked `tail`.
