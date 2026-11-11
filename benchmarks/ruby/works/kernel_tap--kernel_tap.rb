obj = Object.new
x = nil

#--- benchmark: the prelude above runs once, the script below in the loop ---
x = obj.tap { |o| o }
