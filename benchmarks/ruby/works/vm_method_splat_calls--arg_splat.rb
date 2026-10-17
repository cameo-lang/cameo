def f(x=0, y: 0) end
a = [1]
ea = []
kw = {y: 1}
b = lambda{}

#--- benchmark: the prelude above runs once, the script below in the loop ---
f(1, *ea)
