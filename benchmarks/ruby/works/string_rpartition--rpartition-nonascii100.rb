str1 = [*"a".."z",*"0".."9"].join("")
str10 = str1 * 10 + ":"
str100 = str1 * 100 + ":"
str1000 = str1 * 1000 + ":"
nonascii1 = [*"\u{e0}".."\u{ff}"].join("")
nonascii10 = nonascii1 * 10 + ":"
nonascii100 = nonascii1 * 100 + ":"
nonascii1000 = nonascii1 * 1000 + ":"

#--- benchmark: the prelude above runs once, the script below in the loop ---
nonascii100.rpartition(":")
