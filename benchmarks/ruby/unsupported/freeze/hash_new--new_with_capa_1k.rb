has_hash_with_capa = Hash.instance_method(:initialize).parameters.include?([:key, :capacity])
strings_1k = 1_000.times.map { |i| -i.to_s.freeze }
strings_100k = 100_000.times.map { |i| -i.to_s.freeze }

h = has_hash_with_capa ? Hash.new(capacity: strings_1k.size) : {}
strings_1k.each do |x|
  h[x] = true
end

