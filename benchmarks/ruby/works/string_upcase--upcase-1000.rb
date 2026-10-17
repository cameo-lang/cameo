str1 = [*"a".."z",*"0".."9"].join("")
str10 = str1 * 10
str100 = str10 * 10
str1000 = str100 * 10
nonascii1 = [*"\u{e0}".."\u{ff}"].join("")
nonascii10 = nonascii1 * 10
nonascii100 = nonascii10 * 10
nonascii1000 = nonascii100 * 10
ascii8bit256 = (0..255).to_a.pack("C*")
ascii8bit2560 = ascii8bit256 * 10
ascii8bit25600 = ascii8bit2560 * 10
ascii8bit256000 = ascii8bit25600 * 10

#--- benchmark: the prelude above runs once, the script below in the loop ---
str1000.upcase
