small_hash = { a: 1 }
larger_hash = 20.times.map { |i| [('a'.ord + i).chr.to_sym, i] }.to_h

#--- benchmark: the prelude above runs once, the script below in the loop ---
small_hash.dup
