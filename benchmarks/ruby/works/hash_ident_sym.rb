syms = ('a'..'z').to_a.map(&:to_sym)
h = syms.each_with_object({}.compare_by_identity) { |s, acc| acc[s] = s }
200_000.times { syms.each { |s| h[s] } }
