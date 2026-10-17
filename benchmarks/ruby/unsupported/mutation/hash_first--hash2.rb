hash1 = 1_000_000.times.to_h { [rand, true]}
hash2 = hash1.dup
hash2.keys[1..100_000].each { hash2.delete _1 }
hash2.delete hash2.first[0]

hash2.first
