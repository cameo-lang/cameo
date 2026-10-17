small_flat_ary = 5.times.to_a
large_flat_ary = 100.times.to_a
small_pairs_ary = [[1, 2]] * 5
large_pairs_ary = [[1, 2]] * 100
mostly_flat_ary = 100.times.to_a.push([101, 102])
small_nested_ary = [small_flat_ary]
large_nested_ary = [large_flat_ary]

small_flat_ary.flatten
