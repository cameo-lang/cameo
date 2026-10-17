utf8 = "héllo wörld 晦"
broken = "\xff\xfe".dup.force_encoding("UTF-8")

utf8.valid_encoding?
