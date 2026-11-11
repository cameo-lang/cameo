value = 0.01
n = 50_000

h = 1.upto(n).each_with_object({}){|i, h|
  h["%020d" % i] = "v-#{i}"
}

(n * 1_000).times{
  ''
}
