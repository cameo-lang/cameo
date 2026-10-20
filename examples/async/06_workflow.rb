# A workflow of steps that depend on each other. Checking the order and
# pricing its lines run at once; shipping waits for both, and its label is
# made while the parcel is weighed.
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
