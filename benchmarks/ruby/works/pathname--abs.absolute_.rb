abs = Pathname("/a")
rel = Pathname("a")
p1 = Pathname.new('foo/././././bar')
p2 = Pathname.new('foo/bar/./../..')
p3 = Pathname.new('foo/bar/zot')

#--- benchmark: the prelude above runs once, the script below in the loop ---
abs.absolute?
