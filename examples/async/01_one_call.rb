# async starts one method call on a worker and returns its promise, an
# Integer, at once; await waits for the call and returns its result. A class
# includes Cameo::Runner to run its work so.
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
