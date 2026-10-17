objs = 100000.times.map { Object.new }

objs.map(&:freeze)

