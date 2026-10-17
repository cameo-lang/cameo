lstr1 = [*"a".."z",*"0".."9"].join("")
lstr10 = lstr1 * 10
lstr100 = lstr10 * 10
lstr1000 = lstr100 * 10
lnonascii1 = [*"\u{e0}".."\u{ff}"].join("")
lnonascii10 = lnonascii1 * 10
lnonascii100 = lnonascii10 * 10
lnonascii1000 = lnonascii100 * 10
ustr1 = [*"A".."Z",*"0".."9"].join("")
ustr10 = ustr1 * 10
ustr100 = ustr10 * 10
ustr1000 = ustr100 * 10
unonascii1 = [*"\u{c0}".."\u{df}"].join("")
unonascii10 = unonascii1 * 10
unonascii100 = unonascii10 * 10
unonascii1000 = unonascii100 * 10

#--- benchmark: the prelude above runs once, the script below in the loop ---
lstr100.casecmp(ustr100)
