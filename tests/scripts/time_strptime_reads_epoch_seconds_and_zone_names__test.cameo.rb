# time: testcase to demonstrate that time strptime reads epoch seconds and
# zone names
#
# Time.strptime reads %s, seconds since the epoch, with a fraction and a zone
# after it, and %z reads a zone name as well as an offset. Every line prints
# what CRuby 3.3 prints under TZ=UTC.
#
# (Introduced in PR #366. expected output follows the __END__ separator.)
require "time"
[["1.5", "%s.%N"], ["-1.5", "%s.%N"], ["1.123", "%s.%L"], ["1.5 +0900", "%s.%N %z"], ["1709164800", "%s"],
 ["20010203 UTC", "%Y%m%d %z"], ["20010203 GMT", "%Y%m%d %z"], ["20010203 EST", "%Y%m%d %z"], ["20010203 +0900", "%Y%m%d %z"]].each do |s, f|
  t = Time.strptime(s, f)
  puts "#{s} => #{t.strftime("%Y-%m-%d %H:%M:%S.%L %z")}"
end
p [Time.strptime("20010203 UTC", "%Y%m%d %z").utc?, Time.strptime("20010203 EST", "%Y%m%d %z").utc?]

__END__
1.5 => 1970-01-01 00:00:01.500 +0000
-1.5 => 1969-12-31 23:59:58.500 +0000
1.123 => 1970-01-01 00:00:01.123 +0000
1.5 +0900 => 1970-01-01 09:00:01.500 +0900
1709164800 => 2024-02-29 00:00:00.000 +0000
20010203 UTC => 2001-02-03 00:00:00.000 +0000
20010203 GMT => 2001-02-03 00:00:00.000 +0000
20010203 EST => 2001-02-03 00:00:00.000 -0500
20010203 +0900 => 2001-02-03 00:00:00.000 +0900
[true, false]
