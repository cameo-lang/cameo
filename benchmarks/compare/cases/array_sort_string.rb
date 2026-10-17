arr = Array.new(1000) { rand(36**8).to_s(36) }
10000.times { arr.sort }
