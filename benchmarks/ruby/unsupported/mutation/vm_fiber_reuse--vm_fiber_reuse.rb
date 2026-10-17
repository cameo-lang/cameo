GC.disable
fibers = []

1024.times do
  fiber = Fiber.new{Fiber.yield}
  fibers << fiber
  fiber.resume
end

fibers.clear
GC.start

