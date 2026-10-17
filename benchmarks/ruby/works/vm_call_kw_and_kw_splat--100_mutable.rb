h1, h10, h100, h1000 = [1, 10, 100, 1000].map do |n|
  h = {kw: 1}
  n.times{|i| h[i.to_s.to_sym] = i}
  h
end
eh = {}
def kw(kw: nil, **kws) end

kw(**eh, **h100)

