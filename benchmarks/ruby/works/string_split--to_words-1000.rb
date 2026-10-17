str1 = [*0..5].join(" ") + " "
str10 = str1 * 10
str100 = str10 * 10
str1000 = str100 * 10

#--- benchmark: the prelude above runs once, the script below in the loop ---
str1000.split(' ')
