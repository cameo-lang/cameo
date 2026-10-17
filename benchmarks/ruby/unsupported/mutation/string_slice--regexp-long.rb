long_string = "x"*1000+"-hår"

long_string.dup.slice!(/-(.)(.)(.)/, 3)

