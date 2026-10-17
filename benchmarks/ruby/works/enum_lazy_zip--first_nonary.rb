a = (1..3).lazy
b = a.map {|x| x}

a.zip("a".."c").first
