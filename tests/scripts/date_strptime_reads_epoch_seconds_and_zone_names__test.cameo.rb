# date: testcase to demonstrate that date strptime reads epoch seconds and
# zone names
#
# Date._strptime and DateTime.strptime read %s with a fraction and a zone
# after it, and %z reads a zone name. DateTime keeps the seconds and drops the
# fraction, as Ruby's rt_rewrite_frags does. Ruby prints the fraction as the
# Rational (1/2), which cameo leaves out.
#
# The scripts are plain Ruby, and the expected outputs are Ruby 3.4's, with
# date 3.4.1, except where a comment says that Ruby returns a Rational: cameo
# returns an Integer when it is whole and a Float otherwise.
#
# (Introduced in PR #366. expected output follows the __END__ separator.)
require "date"
p Date._strptime("-1.5", "%s.%N")
p Date._strptime("1 +0900", "%s %z")
p Date._strptime("20010203 UTC", "%Y%m%d %z")
p DateTime.strptime("-1.5", "%s.%N")
p DateTime.strptime("1.5 +0900", "%s.%N %z")
p DateTime.strptime("20010203 EST", "%Y%m%d %z")

__END__
{seconds: -1, sec_fraction: 0.5}
{seconds: 1, zone: "+0900", offset: 32400}
{year: 2001, mon: 2, mday: 3, zone: "UTC", offset: 0}
#<DateTime: 1969-12-31T23:59:59+00:00 ((2440587j,86399s,0n),+0s,2299161j)>
#<DateTime: 1970-01-01T09:00:01+09:00 ((2440588j,1s,0n),+32400s,2299161j)>
#<DateTime: 2001-02-03T00:00:00-05:00 ((2451944j,18000s,0n),-18000s,2299161j)>
