ary = (1..10_000).to_a
#--- benchmark: the prelude above runs once, the script below in the loop ---
ary.sample(2)
