
i = 0
str = "same".dup
while i < 10_000_000
  -str
  i += 1
end

