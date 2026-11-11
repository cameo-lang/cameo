strs = ('a'..'z').to_a
h = strs.each_with_object({}.compare_by_identity) { |s, h| h[s] = s }
200_000.times { strs.each { |s| h[s] } }
