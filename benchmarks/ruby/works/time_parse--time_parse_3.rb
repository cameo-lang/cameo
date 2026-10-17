require 'time'
inspect = "2021-08-23 09:57:02 +0900"
iso8601 = "2021-08-23T09:57:02+09:00"

#--- benchmark: the prelude above runs once, the script below in the loop ---
Time.new(iso8601) rescue Time.iso8601(iso8601)
