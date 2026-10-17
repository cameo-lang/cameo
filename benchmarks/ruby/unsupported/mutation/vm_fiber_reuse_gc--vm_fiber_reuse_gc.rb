fibers = []

2000.times do
  fiber = Fiber.new{Fiber.yield}
  fibers << fiber
  fiber.resume
end
fibers.clear

