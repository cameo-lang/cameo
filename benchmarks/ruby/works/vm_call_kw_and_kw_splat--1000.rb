h1, h10, h100, h1000 = [1, 10, 100, 1000].map do |n|
  n.times.each_with_object({kw: 1}){|i, h| h[i.to_s.to_sym] = i}
end
eh = {}
def kw(kw: nil, **kws) end

#--- benchmark: the prelude above runs once, the script below in the loop ---
kw(**h1000)

