str1 = [*"a".."m",*"N".."Z",*"0".."9"].join("")
str10 = str1 * 10
str100 = str10 * 10
str1000 = str100 * 10

str100.dup.capitalize!
