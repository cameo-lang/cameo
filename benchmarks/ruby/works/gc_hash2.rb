value = 0.01
n = 4*(10**6)

h = 1.upto(n).each_with_object({}){|i, h|
  h["%020d" % i] = value * i
}
