i = 0
total = 0
while i < 2_000_000
  s = "ab" + "cd"
  t = s + "efghij" + s
  u = "#{i}:#{t}"
  total += u.size
  i += 1
end
p total
