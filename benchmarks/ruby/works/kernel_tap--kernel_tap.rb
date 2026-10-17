obj = Object.new
x = nil

#--- benchmark: the prelude above runs once, the script below in the loop ---
obj.tap { |o| x = o }
