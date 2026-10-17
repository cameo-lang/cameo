has_hash_with_capa = Hash.instance_method(:initialize).parameters.include?([:key, :capacity])
strings_1k = 1_000.times.map { |i| -i.to_s.freeze }
strings_100k = 100_000.times.map { |i| -i.to_s.freeze }

Hash.new
