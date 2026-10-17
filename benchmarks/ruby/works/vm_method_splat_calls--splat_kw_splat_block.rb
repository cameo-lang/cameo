def f(x=0, y: 0) end
a = [1]
ea = []
kw = {y: 1}
b = lambda{}

f(*a, **kw, &b)
