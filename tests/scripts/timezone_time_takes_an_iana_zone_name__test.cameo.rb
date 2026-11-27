# timezone: testcase to demonstrate that time takes an IANA zone name
#
# An IANA zone name is accepted wherever Ruby takes a UTC offset; Ruby takes a
# zone object there, so these are cameo's own and have no Ruby output to
# match. The instants are fixed, so the output does not depend on the
# machine's zone or clock.
#
# (Introduced in PR #265. expected output follows the __END__ separator.)
t = Time.at(1719835200, in: "Europe/Berlin")
p t, t.zone, t.utc_offset, t.dst?, t.strftime("%Z %z %:z"), t == Time.at(1719835200)
w = Time.new(2024, 1, 15, 12, 0, 0, in: "America/New_York")
p w, w.zone, w.utc_offset, w.dst?, w.utc, w.to_a
p Time.utc(2024, 7, 1, 12).localtime("Asia/Tokyo"), Time.utc(2024, 7, 1, 12).getlocal("Asia/Kolkata").zone
p Time.new(2024, 3, 31, 1, 30, 0, "Europe/Berlin") + 3600, Time.new(2024, 10, 27, 4, 0, 0, in: "Europe/Berlin").utc_offset
p Time.now(in: "Etc/UTC").zone, Time.at(0, in: "+09:00").zone

__END__
2024-07-01 14:00:00 +0200
"CEST"
7200
true
"CEST +0200 +02:00"
true
2024-01-15 12:00:00 -0500
"EST"
-18000
false
2024-01-15 17:00:00 UTC
[0, 0, 12, 15, 1, 2024, 1, 15, false, "EST"]
2024-07-01 21:00:00 +0900
"IST"
2024-03-31 03:30:00 +0200
3600
"UTC"
nil
