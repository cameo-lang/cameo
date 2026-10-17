objs = 100000.times.map { Object.new }

#--- benchmark: the prelude above runs once, the script below in the loop ---
objs.map(&:freeze)

