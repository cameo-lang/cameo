DEFAULT = Object.new
kwargs = {k0: 0}
[31, 32, 128].each do |n|
  [:sparse, :dense].each do |kind|
    params = n.times.map do |i|
      default = kind == :dense || i >= n - 2 ? "DEFAULT" : "0"
      "k#{i}: #{default}"
    end
    eval "def many_#{n}_#{kind}(#{params.join(", ")}); end"
  end
end

many_31_sparse(**kwargs)
