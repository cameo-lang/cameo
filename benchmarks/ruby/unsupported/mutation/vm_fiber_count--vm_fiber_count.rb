fibers = []

fiber = Fiber.new{Fiber.yield}
fibers << fiber
fiber.resume

