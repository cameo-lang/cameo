utf8 = "héllo wörld 晦"
broken = "\xff\xfe".dup.force_encoding("UTF-8")

#--- benchmark: the prelude above runs once, the script below in the loop ---
broken.valid_encoding?
