ascii = "hello world"
utf8 = "héllo wörld 晦"

#--- benchmark: the prelude above runs once, the script below in the loop ---
utf8.ascii_only?
