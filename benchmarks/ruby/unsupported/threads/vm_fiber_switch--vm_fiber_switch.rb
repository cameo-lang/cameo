# based on benchmark for [ruby-core:65518] [Feature #10341] by Knut Franke
fib = Fiber.new do
  loop { Fiber.yield }
end

fib.resume

