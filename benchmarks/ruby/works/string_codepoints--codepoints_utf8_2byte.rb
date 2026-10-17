mixed_ascii64 = ("a" * 63 + "\u{100}") * 2048
mixed_ascii256 = ("a" * 255 + "\u{100}") * 512
utf8_2byte = "\u{100}" * 65536

#--- benchmark: the prelude above runs once, the script below in the loop ---
utf8_2byte.codepoints
