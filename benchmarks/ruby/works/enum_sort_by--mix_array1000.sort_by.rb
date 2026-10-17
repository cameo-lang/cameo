array_length = 2
fixnum_array2 = array_length.times.to_a.map {rand(10000)}
float_array2 = array_length.times.to_a.map {rand(10000.0).to_f}
string_array2 = array_length.times.to_a.map {"r" * rand(1..10000)}
mix_array2 = array_length.times.to_a.map {if rand(1..100) <= 50 then rand(1..10000).to_f else rand(1..10000) end}
all_zero_array2 =array_length.times.to_a.map {0}

array_length = 10
fixnum_array10 = array_length.times.to_a.map {rand(10000)}
float_array10 = array_length.times.to_a.map {rand(10000.0).to_f}
string_array10 = array_length.times.to_a.map {"r" * rand(1..10000)}
mix_array10 = array_length.times.to_a.map {if rand(1..100) <= 50 then rand(1..10000).to_f else rand(1..10000) end}
all_zero_array10 =array_length.times.to_a.map {0}

array_length = 1000
fixnum_array1000 = array_length.times.to_a.map {rand(10000)}
float_array1000 = array_length.times.to_a.map {rand(10000.0).to_f}
string_array1000 = array_length.times.to_a.map {"r" * rand(1..10000)}
mix_array1000 = array_length.times.to_a.map {if rand(1..100) <= 50 then rand(1..10000).to_f else rand(1..10000) end}
all_zero_array1000 =array_length.times.to_a.map {0}

array_length = 100000
fixnum_array100000 = array_length.times.to_a.map {rand(10000)}
float_array100000 = array_length.times.to_a.map {rand(10000.0).to_f}
string_array100000 = array_length.times.to_a.map {"r" * rand(1..10000)}
mix_array100000 = array_length.times.to_a.map {if rand(1..100) <= 50 then rand(1..10000).to_f else rand(1..10000) end}
all_zero_array100000 =array_length.times.to_a.map {0}

mix_array1000.sort_by {|a| a}
