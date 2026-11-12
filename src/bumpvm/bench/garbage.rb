def make(i)
  [i.to_s, "item-" + i.to_s, [i, i * 2]]
end
i = 0
total = 0
keep = []
while i < 2_000_000
  x = make(i)
  keep = x if i % 1000 == 0
  total += x[1].size + x[2][1]
  i += 1
end
p total, keep[1]
