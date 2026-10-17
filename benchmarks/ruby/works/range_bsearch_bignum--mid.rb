first = 2**100
last = 2**1000
mid = (first + last) / 2
r = first..last

r.bsearch { |x| x >= mid }
