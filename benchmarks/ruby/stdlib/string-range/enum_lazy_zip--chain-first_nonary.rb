a = (1..3).lazy
b = a.map {|x| x}

b.zip("a".."c").first
