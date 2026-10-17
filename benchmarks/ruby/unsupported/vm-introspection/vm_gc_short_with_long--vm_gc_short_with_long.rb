long_lived = Array.new(1_000_000){|i| "#{i}"}
GC.start
GC.start

a = '' # short-lived String
b = ''
c = ''
d = ''
e = ''
f = ''

