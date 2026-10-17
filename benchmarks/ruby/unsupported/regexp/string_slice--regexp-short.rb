long_string = "x"*1000+"-hår"

"x-hår".slice!(/-(.)(.)(.)/, 3)

