a = (1..3).lazy
b = a.map {|x| x}

#--- benchmark: the prelude above runs once, the script below in the loop ---
a.zip(["a", "b", "c"]).take(3).force
