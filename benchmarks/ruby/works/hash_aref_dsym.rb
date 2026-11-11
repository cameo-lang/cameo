syms = ('a'..'z').map { |s| s.to_sym }
h = syms.each_with_object({}) { |s, acc| acc[s] = 1 }
200_000.times { syms.each { |s| h[s] } }
