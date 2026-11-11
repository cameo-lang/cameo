h = 10000.times.each_with_object({}) do |i, h|
  h[i] = nil
end

5000.times do
  h.values
end
