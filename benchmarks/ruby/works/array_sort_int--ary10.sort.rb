ary2 = 2.times.to_a.shuffle
ary10 = 10.times.to_a.shuffle
ary100 = 100.times.to_a.shuffle
ary1000 = 1000.times.to_a.shuffle
ary10000 = 10000.times.to_a.shuffle

#--- benchmark: the prelude above runs once, the script below in the loop ---
ary10.sort
