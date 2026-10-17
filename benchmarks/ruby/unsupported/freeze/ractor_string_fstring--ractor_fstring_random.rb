
i = 0
str = "same".dup
while i < 2000000
  -(i.to_s.freeze)
  i += 1
end

