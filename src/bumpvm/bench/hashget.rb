h = { 1 => 10, 2 => 20, 3 => 30, 4 => 40, 5 => 50, 6 => 60, 7 => 70, 8 => 80 }
g = { "alpha" => 1, "beta" => 2, "gamma_delta" => 3, :sym => 4 }
i = 0
total = 0
while i < 5_000_000
  total += h[i % 8 + 1] + g["beta"] + g["gamma_delta"] + g[:sym]
  i += 1
end
p total
