obj = Object.new
hash = { obj => true }

#--- benchmark: the prelude above runs once, the script below in the loop ---
hash.key?(obj)
