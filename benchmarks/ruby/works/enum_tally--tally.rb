list = ("aaa".."zzz").to_a*10

#--- benchmark: the prelude above runs once, the script below in the loop ---
list.tally
