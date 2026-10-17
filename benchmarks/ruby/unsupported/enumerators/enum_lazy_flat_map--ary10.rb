num = (1..).lazy.take(100)
ary2 = [[1,2]].cycle.lazy.take(10)
ary10 = [[*1..10]].cycle.lazy.take(10)
ary20 = [[*1..20]].cycle.lazy.take(10)
ary50 = [[*1..50]].cycle.lazy.take(10)
ary100 = [[*1..100]].cycle.lazy.take(10)

ary10.flat_map {|x| x}.take(3).to_a
