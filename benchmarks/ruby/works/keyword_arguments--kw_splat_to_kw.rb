h = {a: 1}
def kw(a: 1) a end
def kws(**kw) kw end

#--- benchmark: the prelude above runs once, the script below in the loop ---
kw(**h)
