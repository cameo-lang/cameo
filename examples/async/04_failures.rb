# A call that raises raises again where it is awaited, with the same class
# and message. Awaiting a list waits for every call, then raises the first
# failure in the list's order.
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
