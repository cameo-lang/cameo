define_method(:a0){}
define_method(:a1){|a| a}
define_method(:s){|*a| a}
define_method(:b){|kw: 1| kw}

t0 = 0.times.to_a
t1 = 1.times.to_a
t10 = 10.times.to_a
t100 = 100.times.to_a
kw = {kw: 2}

b(**kw)

