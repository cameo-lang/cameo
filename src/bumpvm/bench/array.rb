i = 0
total = 0
while i < 3_000_000
  a = [i, i + 1, i + 2, "x"]
  b = [a, a[1]]
  total += b[0][2] + b[1] + a.size
  i += 1
end
p total
