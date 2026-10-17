rf_1 = 0..1
rf_1k = 0..1000
rf_1m = 0..1000000
big = 2**1000
rb_1 = big..big+1
rb_1k = big..big+1000
rb_1m = big..big+1000000

rb_1m.reverse_each { _1 }
