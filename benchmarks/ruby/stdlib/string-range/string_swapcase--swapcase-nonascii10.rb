str1 = [*"A".."M",*"n".."z",*"0".."9"].join("")
str10 = str1 * 10
str100 = str10 * 10
str1000 = str100 * 10
nonascii1 = [*"\u{c0}".."\u{cf}",*"\u{f0}".."\u{ff}"].join("")
nonascii10 = nonascii1 * 10
nonascii100 = nonascii10 * 10
nonascii1000 = nonascii100 * 10

nonascii10.swapcase
