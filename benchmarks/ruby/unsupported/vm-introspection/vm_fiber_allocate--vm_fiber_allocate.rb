# Disable GC to see raw throughput:
GC.disable

fiber = Fiber.new{Fiber.yield}
fiber.resume

