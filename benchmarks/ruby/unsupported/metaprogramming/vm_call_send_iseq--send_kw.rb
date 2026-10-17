def a0; end
def a1(a) a; end
def s(*a) a; end
def b(kw: 1) kw end
def sb(*a, kw: 1) kw end

t0 = 0.times.to_a
t1 = 1.times.to_a
t10 = 10.times.to_a
t200 = 200.times.to_a

a0_t0 = [:a0, *t0]
a1_t1 = [:a1, *t1]
s_t0 = [:s, *t0]
s_t1 = [:s, *t1]
s_t10 = [:s, *t10]
s_t200 = [:s, *t200]
sb_t0 = [:sb, *t0]
sb_t1 = [:sb, *t1]
sb_t10 = [:sb, *t10]
sb_t200 = [:sb, *t200]
kw = {kw: 2}

send(:b, kw: 1)

