# class names and core classes: testcase to demonstrate that `attr` takes
# the name of a class (`type: "Shop::Item"`) as well as a class, that core
# classes type a field, and that a value of the wrong class raises a TypeError
#
# (Introduced in PR #465. expected output follows the __END__ separator.)
module Shop
  class Item
    string :sku
  end
end

class Line
  attr :item, type: "Shop::Item"
  attr :qty, type: Integer, default: 1
  attr :label, type: String
  attr :price, :tax, type: Float
end

l = Line.new(item: Shop::Item.new(sku: "x"), label: "a", price: 2)
p l.item.sku, l.qty, l.label, l.price
p l.with { |x| { qty: x.qty + 2 } }.qty

begin
  Line.new(qty: "no")
rescue TypeError => e
  puts e.message
end

begin
  Line.new(item: "no")
rescue TypeError => e
  puts e.message
end

__END__
"x"
1
"a"
2.0
3
field qty of Line takes an int, not an instance of String
field item of Line takes a Shop::Item, not an instance of String
