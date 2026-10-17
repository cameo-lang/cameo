str1 = [*"a".."m",*"N".."Z",*"0".."9"].join("")
str10 = str1 * 10
str100 = str10 * 10
str1000 = str100 * 10

#--- benchmark: the prelude above runs once, the script below in the loop ---
str10.capitalize
