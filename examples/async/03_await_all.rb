# await :all waits for every call the current method started and has not
# awaited, blocks included, and returns their results in start order.
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
