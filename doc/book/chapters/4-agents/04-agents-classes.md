## Classes: typed fields and `with` {#ch-agents-classes}

::: source
from `src/agentdocs/topics/classes.md`
:::

A class declares its fields with their types. `new` takes them by name when
the class has no `initialize`. A method that would change the instance returns
a changed copy with `with`, and the caller assigns it.

``` ruby
class Order
  string :customer
  list :items, of: :string
  int :total_cents, default: 0

  def add(item, cents)
    with(items: items + [item], total_cents: total_cents + cents)
  end

  def to_s
    "#{customer}: #{items.join(", ")} (#{format("%.2f", total_cents / 100.0)})"
  end
end

order = Order.new(customer: "Ada", items: [])
order = order.add("tea", 350).add("cake", 500)
puts order                 # Ada: tea, cake (8.50)
```

The field types are `int`, `float`, `string`, `bool`, `bytes`, protobuf's
sized numbers (`int32`, `uint64`, …), `list` and `set` with `of:`, `map`,
`ref :name, OtherClass` and `enum`. A wrong type raises `TypeError`. Inside
`initialize`, set a field with `self.x = v`. Every object is a protobuf
message: `cameo proto script.cameo` prints the `.proto` files of a script's
classes.
