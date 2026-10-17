50_000.times{|i| sym = "sym#{i}".to_sym}
GC.start
GC.start

a = '' # short-lived String
b = ''
c = ''
d = ''
e = ''
f = ''

