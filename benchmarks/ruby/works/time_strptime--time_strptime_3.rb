require 'time'

#--- benchmark: the prelude above runs once, the script below in the loop ---
Time.strptime("0 UTC", "%s %z")
