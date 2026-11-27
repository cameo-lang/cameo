# timezone: testcase to demonstrate that time refuses a zone name it does not
# know
#
# An IANA zone name is accepted wherever Ruby takes a UTC offset; Ruby takes a
# zone object there, so these are cameo's own and have no Ruby output to
# match. The instants are fixed, so the output does not depend on the
# machine's zone or clock.
#
# (Introduced in PR #265. expected output follows the __END__ separator.)
begin; Time.at(0, in: "Europe/Nowhere"); rescue ArgumentError => e; puts e.message; end
begin; Time.at(0, in: "CET"); rescue ArgumentError => e; puts e.message; end
begin; Time.utc(2024, 1, 1).localtime("Mars/Olympus"); rescue ArgumentError => e; puts e.message; end

__END__
unknown time zone Europe/Nowhere: an IANA name such as Europe/Berlin, or an offset such as +01:00, expected
"+HH:MM", "-HH:MM", "UTC" or "A".."I","K".."Z" expected for utc_offset: CET
unknown time zone Mars/Olympus: an IANA name such as Europe/Berlin, or an offset such as +01:00, expected
