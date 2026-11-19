# Collections without accumulators

A block captures the values around it and cannot assign to a variable
outside itself. Reach for the method that returns the result.

```ruby
words = %w[tea cake tea scone tea]

counts = words.tally                                   # {"tea" => 3, "cake" => 1, "scone" => 1}
total = words.sum(&:length)                            # 18
longest = words.reduce { |best, w| w.length > best.length ? w : best }
index = words.each_with_object({}) { |w, seen| seen[w] = true }.keys

i = 0                      # a counter lives in a while loop
found = nil
while i < words.size && found.nil?
  found = i if words[i] == "scone"
  i += 1
end
```

`h[k] = v` and `a.x = 1` are assignments: they rebind `h` and `a` to a
changed copy. Any other variable that held the old value keeps it.
