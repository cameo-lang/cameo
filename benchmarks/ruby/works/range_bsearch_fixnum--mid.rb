first = 1
last = 10000
mid = (first + last) / 2
r = first..last

r.bsearch { |x| x >= mid }
