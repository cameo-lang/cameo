require 'time'

#--- benchmark: the prelude above runs once, the script below in the loop ---
Time.strptime("1.5", "%s.%N")
