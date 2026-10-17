require 'time'

#--- benchmark: the prelude above runs once, the script below in the loop ---
Time.strptime("20010203 UTC", "%Y%m%d %z")
