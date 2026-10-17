def self.a0; end
def self.a1(a) a; end
def self.s(*a) a; end
def self.b(kw: 1) kw end
def self.sb(*a, kw: 1) kw end

t0 = 0.times.to_a
t1 = 1.times.to_a
t10 = 10.times.to_a
t200 = 200.times.to_a

a0_t0 = [self, *t0]
a1_t1 = [self, *t1]
s_t0 = [self, *t0]
s_t1 = [self, *t1]
s_t10 = [self, *t10]
s_t200 = [self, *t200]
sb_t0 = [self, *t0]
sb_t1 = [self, *t1]
sb_t10 = [self, *t10]
sb_t200 = [self, *t200]
kw = {kw: 2}

a0 = :a0.to_proc
a1 = :a1.to_proc
s = :s.to_proc
b = :b.to_proc
sb = :sb.to_proc

#--- benchmark: the prelude above runs once, the script below in the loop ---
s.(*s_t0)

