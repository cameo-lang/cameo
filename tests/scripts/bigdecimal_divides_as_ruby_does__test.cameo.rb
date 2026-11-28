# bigdecimal: testcase to demonstrate that big decimal divides as Ruby does
#
# The quotients have the digits bigdecimal computes, its last digit rounded
# from its own estimate of the next; sqrt differs as above.
#
# (Introduced in PR #290. expected output follows the __END__ separator.)
require "bigdecimal"
require "bigdecimal/util"
pairs = [["1","3"],["10","3"],["100","3"],["1e9","3"],["1e10","3"],["0.1","3"],["0.001","3"],["1","0.3"],["1","300"],["1","3e9"],["123456789","3"],["1234567890","3"],["12345678901234567890","3"],["1","123456789"],["1","1234567890"],["1","12345678901234567890"],["4.5","1.23"],["10","1.23"],["2","7"],["20","7"],["200","7"],["0.2","7"],["123456789.123456789","7"],["1","7e-9"],["1e-9","7"],["1","1180591620717411303424"],["1180591620717411303424","3"],["1.23","4.5"],["1","1.23"],["100","1.23"],["1000000000","1.23"],["0.000000001","1.23"],["1.5","0.000000007"],["123456789012345678901234567890123456789","7"],["1","123456789012345678901234567890123456789"],["99999999999999999999","99999999999999999998"],["1.0000000001","3"]]
pairs.each do |a, b|
  q = BigDecimal(a) / BigDecimal(b)
  puts "#{a} / #{b} = #{q} digits=#{q.n_significant_digits} exp=#{q.exponent}"
end
puts "--- div(n)"
[[ "1","3",1],["1","3",2],["1","3",9],["1","3",10],["2","3",1],["4.5","1.23",3],["4.5","1.23",10],["1","3",0],["22","7",3]].each { |a,b,n| puts "#{a}.div(#{b}, #{n}) = #{BigDecimal(a).div(BigDecimal(b), n)}" }
puts "--- quo/fdiv"
puts BigDecimal("1").quo(BigDecimal("3")), BigDecimal("1").quo(3), BigDecimal("1").fdiv(3)
puts "--- power neg"
[["1.23",-1],["3",-1],["7",-2],["1.5",-3],["2",-10]].each { |a,n| q = BigDecimal(a) ** n; puts "#{a} ** #{n} = #{q} digits=#{q.n_significant_digits}" }
puts "--- float pow"
[["2",0.5],["4",0.5],["1.5",2.5],["2",1.5],["10",0.1]].each { |a,n| q = BigDecimal(a) ** n; puts "#{a} ** #{n} = #{q} digits=#{q.n_significant_digits}" }
puts "--- sqrt"
[["2",1],["2",5],["2",10],["2",20],["3",10],["10",10],["0.5",10],["16",10],["1e20",10]].each { |a,n| q = BigDecimal(a).sqrt(n); puts "#{a}.sqrt(#{n}) = #{q} digits=#{q.n_significant_digits}" }
puts "--- mult/add prec"
[["1.23","4.5",0],["1.23","4.5",1],["1.23","4.5",2],["1.23","4.5",3],["1.23","4.5",4],["1.23","4.5",5]].each { |a,b,n| puts "#{a}.mult(#{b}, #{n}) = #{BigDecimal(a).mult(BigDecimal(b), n)}  add=#{BigDecimal(a).add(BigDecimal(b), n)}  sub=#{BigDecimal(a).sub(BigDecimal(b), n)}" }
puts "--- float to bigdecimal"
[[1.5, 0], [0.1, 0], [1.0/3, 0], [1.0/3, 5], [123.456, 0], [1e20, 0], [1.1, 20], [0.1, 15], [0.1, 16], [0.1, 17], [0.1, 18]].each { |f, n| begin; puts "BigDecimal(#{f}, #{n}) = #{BigDecimal(f, n)}"; rescue => e; puts "BigDecimal(#{f}, #{n}) = #{e.class}: #{e.message}"; end }
[1.5, 0.1, 1.0/3, 123.456, 1e20, 1e-5, 2.5e-7, 1e100, 1.7976931348623157e308].each { |f| puts "#{f}.to_d = #{f.to_d}  to_d(3)=#{f.to_d(3)}" }
puts "--- to_s F forms"
["0.000123", "123000", "1.5e-10", "1e-1", "0.1e1", "-123.456e5", "12345678901234567890.123456789"].each { |s| d = BigDecimal(s); puts "#{s}: to_s=#{d} F=#{d.to_s("F")} 3F=#{d.to_s("3F")} digits=#{d.to_digits} exp=#{d.exponent} prec=#{d.precision} scale=#{d.scale} split=#{d.split.inspect}" }
puts "--- parsing"
["1.", "+.5", "-.5", "1e+2", "1E-2", "1_0.5_5", "  12  ", "12abc", "1 e2", "1.5e", "e5", "Infinity ", "-Infinity", "+Infinity", "nan", "inf", "1e1000000000", "1e-1000000000", "0.0e5", "00012.5000", "1__0"].each { |s| begin; puts "#{s.inspect} => #{BigDecimal(s)}"; rescue => e; puts "#{s.inspect} => #{e.class}: #{e.message}"; end }
puts "--- integer ops"
puts (BigDecimal("7") % 2), (BigDecimal("-7") % 2), (BigDecimal("7") % -2), BigDecimal("7").remainder(-2), BigDecimal("-7").remainder(2), BigDecimal("7.5").divmod(2).inspect, BigDecimal("-7.5").divmod(2).inspect, BigDecimal("7.5").div(2), BigDecimal("-7.5").div(2), BigDecimal("7.5").modulo(BigDecimal("2.5")), BigDecimal("1").div(BigDecimal("0.3"))
puts "--- limit"
BigDecimal.limit(5)
puts BigDecimal("1.23456789"), BigDecimal("1.23456789") + 0, BigDecimal("1") / 3, BigDecimal("123456.789") * 1, BigDecimal("1.23456789").add(0, 8), BigDecimal("12345678") + 1, BigDecimal("1.23456") * BigDecimal("1.23456")
BigDecimal.limit(0)
puts "--- rounding of exact ops"
puts BigDecimal("0.1") * BigDecimal("0.2"), BigDecimal("1e-5") * BigDecimal("1e-5"), BigDecimal("123456789.123456789") * BigDecimal("987654321.987654321"), BigDecimal("1e30") + BigDecimal("1e-30")
puts "--- round with digits beyond"
puts BigDecimal("1.23456").round(10), BigDecimal("1.23456").floor(10), BigDecimal("12345").round(-10), BigDecimal("12345").floor(-2), BigDecimal("12345").ceil(-2), BigDecimal("-12345").truncate(-2), BigDecimal("0").round(2), BigDecimal("1.5").round(0, :banker)

__END__
1 / 3 = 0.333333333333333333333333333333333333e0 digits=36 exp=0
10 / 3 = 0.3333333333333333333333333333333333333e1 digits=37 exp=1
100 / 3 = 0.33333333333333333333333333333333333333e2 digits=38 exp=2
1e9 / 3 = 0.333333333333333333333333333333333333e9 digits=36 exp=9
1e10 / 3 = 0.3333333333333333333333333333333333333e10 digits=37 exp=10
0.1 / 3 = 0.33333333333333333333333333333333333333333333e-1 digits=44 exp=-1
0.001 / 3 = 0.333333333333333333333333333333333333333333e-3 digits=42 exp=-3
1 / 0.3 = 0.3333333333333333333333333333e1 digits=28 exp=1
1 / 300 = 0.3333333333333333333333333333333333e-2 digits=34 exp=-2
1 / 3e9 = 0.333333333333333333333333333333333333e-9 digits=36 exp=-9
123456789 / 3 = 0.41152263e8 digits=8 exp=8
1234567890 / 3 = 0.41152263e9 digits=8 exp=9
12345678901234567890 / 3 = 0.411522630041152263e19 digits=18 exp=19
1 / 123456789 = 0.8100000073710000670761006104e-8 digits=28 exp=-8
1 / 1234567890 = 0.810000007371000067076100610392515555e-9 digits=36 exp=-9
1 / 12345678901234567890 = 0.810000007290000066347100603757805494196111e-19 digits=42 exp=-19
4.5 / 1.23 = 0.3658536585365853658536585365853658537e1 digits=37 exp=1
10 / 1.23 = 0.8130081300813008130081300813008130082e1 digits=37 exp=1
2 / 7 = 0.285714285714285714285714285714285714e0 digits=36 exp=0
20 / 7 = 0.2857142857142857142857142857142857143e1 digits=37 exp=1
200 / 7 = 0.28571428571428571428571428571428571429e2 digits=38 exp=2
0.2 / 7 = 0.28571428571428571428571428571428571428571429e-1 digits=44 exp=-1
123456789.123456789 / 7 = 0.17636684160493827e8 digits=17 exp=8
1 / 7e-9 = 0.142857142857142857142857142857142857e9 digits=36 exp=9
1e-9 / 7 = 0.142857142857142857142857142857142857e-9 digits=36 exp=-9
1 / 1180591620717411303424 = 0.847032947254300339068322500679641962051392e-21 digits=42 exp=-21
1180591620717411303424 / 3 = 0.393530540239137101141333333333333333333333333333e21 digits=48 exp=21
1.23 / 4.5 = 0.273333333333333333333333333333333333e0 digits=36 exp=0
1 / 1.23 = 0.813008130081300813008130081300813009e0 digits=36 exp=0
100 / 1.23 = 0.81300813008130081300813008130081300814e2 digits=38 exp=2
1000000000 / 1.23 = 0.813008130081300813008130081300813009e9 digits=36 exp=9
0.000000001 / 1.23 = 0.813008130081300813008130081300813009e-9 digits=36 exp=-9
1.5 / 0.000000007 = 0.214285714285714285714285714285714286e9 digits=36 exp=9
123456789012345678901234567890123456789 / 7 = 0.17636684144620811271604938270017636684142857142857142857142857142857142857142857143e38 digits=83 exp=38
1 / 123456789012345678901234567890123456789 = 0.8100000072900000663390006036849054935327209911477529194445515669454192592033153e-38 digits=79 exp=-38
99999999999999999999 / 99999999999999999998 = 0.1000000000000000000010000000000000000000200001e1 digits=46 exp=1
1.0000000001 / 3 = 0.333333333366666666666666666666666667e0 digits=36 exp=0
--- div(n)
1.div(3, 1) = 0.3e0
1.div(3, 2) = 0.33e0
1.div(3, 9) = 0.333333333e0
1.div(3, 10) = 0.3333333333e0
2.div(3, 1) = 0.7e0
4.5.div(1.23, 3) = 0.366e1
4.5.div(1.23, 10) = 0.3658536585e1
1.div(3, 0) = 0.333333333333333333333333333333333333e0
22.div(7, 3) = 0.314e1
--- quo/fdiv
0.333333333333333333333333333333333333e0
0.333333333333333333333333333333333333e0
0.3333333333333333
--- power neg
1.23 ** -1 = 0.8130081300813008130081300813008130081300813e0 digits=43
3 ** -1 = 0.333333333333333333333333333e0 digits=27
7 ** -2 = 0.20408163265306122448979591836734693e-1 digits=35
1.5 ** -3 = 0.296296296296296296296296296296296296296296296296296296296296296296296296296296296e0 digits=81
2 ** -10 = 0.9765625e-3 digits=7
--- float pow
2 ** 0.5 = 0.1414213562373095048801689e1 digits=25
4 ** 0.5 = 0.2e1 digits=1
1.5 ** 2.5 = 0.2755675960631075360471944584044128e1 digits=34
2 ** 1.5 = 0.2828427124746190097603377e1 digits=25
10 ** 0.1 = 0.1258925411794167210423954e1 digits=25
--- sqrt
2.sqrt(1) = 0.1414213562373095e1 digits=16
2.sqrt(5) = 0.1414213562373095e1 digits=16
2.sqrt(10) = 0.1414213562373095e1 digits=16
2.sqrt(20) = 0.14142135623730950488e1 digits=20
3.sqrt(10) = 0.1732050807568877e1 digits=16
10.sqrt(10) = 0.3162277660168379e1 digits=16
0.5.sqrt(10) = 0.7071067811865475e0 digits=16
16.sqrt(10) = 0.4e1 digits=1
1e20.sqrt(10) = 0.1e11 digits=1
--- mult/add prec
1.23.mult(4.5, 0) = 0.5535e1  add=0.573e1  sub=-0.327e1
1.23.mult(4.5, 1) = 0.6e1  add=0.6e1  sub=-0.3e1
1.23.mult(4.5, 2) = 0.55e1  add=0.57e1  sub=-0.33e1
1.23.mult(4.5, 3) = 0.554e1  add=0.573e1  sub=-0.327e1
1.23.mult(4.5, 4) = 0.5535e1  add=0.573e1  sub=-0.327e1
1.23.mult(4.5, 5) = 0.5535e1  add=0.573e1  sub=-0.327e1
--- float to bigdecimal
BigDecimal(1.5, 0) = 0.15e1
BigDecimal(0.1, 0) = 0.1e0
BigDecimal(0.3333333333333333, 0) = 0.3333333333333333e0
BigDecimal(0.3333333333333333, 5) = 0.33333e0
BigDecimal(123.456, 0) = 0.123456e3
BigDecimal(1.0e+20, 0) = 0.1e21
BigDecimal(1.1, 20) = ArgumentError: precision too large.
BigDecimal(0.1, 15) = 0.1e0
BigDecimal(0.1, 16) = 0.1e0
BigDecimal(0.1, 17) = ArgumentError: precision too large.
BigDecimal(0.1, 18) = ArgumentError: precision too large.
1.5.to_d = 0.15e1  to_d(3)=0.15e1
0.1.to_d = 0.1e0  to_d(3)=0.1e0
0.3333333333333333.to_d = 0.3333333333333333e0  to_d(3)=0.333e0
123.456.to_d = 0.123456e3  to_d(3)=0.123e3
1.0e+20.to_d = 0.1e21  to_d(3)=0.1e21
1.0e-05.to_d = 0.1e-4  to_d(3)=0.1e-4
2.5e-07.to_d = 0.25e-6  to_d(3)=0.25e-6
1.0e+100.to_d = 0.1e101  to_d(3)=0.1e101
1.7976931348623157e+308.to_d = 0.1797693134862315e309  to_d(3)=0.18e309
--- to_s F forms
0.000123: to_s=0.123e-3 F=0.000123 3F=0.000 123 digits=0.000123 exp=-3 prec=6 scale=6 split=[1, "123", 10, -3]
123000: to_s=0.123e6 F=123000.0 3F=123 000.0 digits=123000.0 exp=6 prec=6 scale=0 split=[1, "123", 10, 6]
1.5e-10: to_s=0.15e-9 F=0.00000000015 3F=0.000 000 000 15 digits=0.00000000015 exp=-9 prec=11 scale=11 split=[1, "15", 10, -9]
1e-1: to_s=0.1e0 F=0.1 3F=0.1 digits=0.1 exp=0 prec=1 scale=1 split=[1, "1", 10, 0]
0.1e1: to_s=0.1e1 F=1.0 3F=1.0 digits=1.0 exp=1 prec=1 scale=0 split=[1, "1", 10, 1]
-123.456e5: to_s=-0.123456e8 F=-12345600.0 3F=-12 345 600.0 digits=-12345600.0 exp=8 prec=8 scale=0 split=[-1, "123456", 10, 8]
12345678901234567890.123456789: to_s=0.12345678901234567890123456789e20 F=12345678901234567890.123456789 3F=12 345 678 901 234 567 890.123 456 789 digits=12345678901234567890.123456789 exp=20 prec=29 scale=9 split=[1, "12345678901234567890123456789", 10, 20]
--- parsing
"1." => ArgumentError: invalid value for BigDecimal(): "1."
"+.5" => 0.5e0
"-.5" => -0.5e0
"1e+2" => 0.1e3
"1E-2" => 0.1e-1
"1_0.5_5" => 0.1055e2
"  12  " => 0.12e2
"12abc" => ArgumentError: invalid value for BigDecimal(): "12abc"
"1 e2" => ArgumentError: invalid value for BigDecimal(): "1 e2"
"1.5e" => ArgumentError: invalid value for BigDecimal(): "1.5e"
"e5" => ArgumentError: invalid value for BigDecimal(): "e5"
"Infinity " => Infinity
"-Infinity" => -Infinity
"+Infinity" => Infinity
"nan" => ArgumentError: invalid value for BigDecimal(): "nan"
"inf" => ArgumentError: invalid value for BigDecimal(): "inf"
"1e1000000000" => 0.1e1000000001
"1e-1000000000" => 0.1e-999999999
"0.0e5" => 0.0
"00012.5000" => 0.125e2
"1__0" => ArgumentError: invalid value for BigDecimal(): "1__0"
--- integer ops
0.1e1
0.1e1
-0.1e1
0.1e1
-0.1e1
[0.3e1, 0.15e1]
[-0.4e1, 0.5e0]
3
-4
0.0
3
--- limit
0.123456789e1
0.12346e1
0.33333e0
0.12346e6
0.12345679e1
0.12346e8
0.15241e1
--- rounding of exact ops
0.2e-1
0.1e-9
0.121932631356500531347203169112635269e18
0.1000000000000000000000000000000000000000000000000000000000001e31
--- round with digits beyond
0.123456e1
0.123456e1
0
0.123e5
0.124e5
-0.123e5
0.0
0.2e1
