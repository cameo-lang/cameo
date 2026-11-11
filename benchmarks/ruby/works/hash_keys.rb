h = 10000.times.each_with_object({}) do |i, acc|
  acc[i] = nil
end

5000.times do
  h.keys
end
