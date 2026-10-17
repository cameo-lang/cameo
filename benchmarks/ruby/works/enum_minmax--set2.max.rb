set2 = 2.times.to_a.shuffle.to_set
set10 = 10.times.to_a.shuffle.to_set
set100 = 100.times.to_a.shuffle.to_set
set1000 = 1000.times.to_a.shuffle.to_set
set10000 = 10000.times.to_a.shuffle.to_set

#--- benchmark: the prelude above runs once, the script below in the loop ---
set2.max
