require 'time'

#--- benchmark: the prelude above runs once, the script below in the loop ---
Time.strptime("20010203 -0200", "%Y%m%d %z")
