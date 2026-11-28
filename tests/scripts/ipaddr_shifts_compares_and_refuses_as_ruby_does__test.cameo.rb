# ipaddr: testcase to demonstrate that IP addr shifts compares and refuses as
# Ruby does
#
# Shifts, comparisons, inclusion, shortening, prefixes, zones and overflow
# follow ipaddr 1.2.7: a negative shift count shifts the other way, == and <=>
# take any Integer, include? and to_range use the masked address, ::ffff:1 is
# ::255.255.0.1, a prefix with leading zeros is refused, a zone id is word
# characters and part of eql?, an address past the family is refused rather
# than wrapped, and ipv4_compat keeps the IPv4 mask. The messages that name an
# address are cameo's.
#
# The expected strings are Ruby's, with ipaddr 1.2.7, but for the error
# messages: Ruby 3.4.1's name no address, and cameo's name the address or the
# length that was refused.
#
# (Introduced in PR #274. expected output follows the __END__ separator.)
require "ipaddr"
p (IPAddr.new("192.168.1.10") << -1).to_s, (IPAddr.new("192.168.1.10") << 1).to_s
v6 = IPAddr.new("2001:db8::1"); p v6 == v6.to_i, v6 <=> v6.to_i
p IPAddr.new("192.168.1.0/24").succ.include?("192.168.1.5"), IPAddr.new("192.168.1.0/24").succ.to_range.first.to_s
p IPAddr.new("::ffff:1").to_s, IPAddr.new("::ffff:1:2").to_s, IPAddr.new("::ffff:0:1:2").to_s, IPAddr.new("::1:2").to_s
["1.2.3.4/024", "fe80::1%", "fe80::1%eth-0", "fe80::1%eth_0"].each do |s|
  p IPAddr.new(s).to_s
rescue IPAddr::Error => e
  puts "#{e.class}: #{e.message}"
end
begin; IPAddr.new("255.255.255.255").succ; rescue IPAddr::InvalidAddressError => e; puts e.message; end
begin; IPAddr.new("192.168.1.10") | IPAddr.new("2001:db8::1"); rescue IPAddr::InvalidAddressError => e; puts e.message; end
begin; IPAddr.new("192.168.1.10") >> -1; rescue IPAddr::InvalidAddressError => e; puts e.message; end
p IPAddr.new("192.168.1.0/24").ipv4_compat.inspect, IPAddr.new("192.168.1.0/24").ipv4_compat.prefix, IPAddr.new("fe80::1%eth0").eql?(IPAddr.new("fe80::1"))
begin; IPAddr.ntop(5); rescue TypeError => e; puts e.message; end

__END__
"96.84.0.133"
"129.80.2.20"
true
0
true
"192.168.1.0"
"::255.255.0.1"
"::ffff:0.1.0.2"
"::ffff:0:1:2"
"::0.1.0.2"
IPAddr::InvalidPrefixError: leading zeros in prefix
IPAddr::InvalidAddressError: invalid address: fe80::1%
IPAddr::InvalidAddressError: invalid address: fe80::1%eth-0
"fe80::1%eth_0"
invalid address: 4294967296
invalid address: 42540766411282592856903984954886062347
invalid address: 6464471572
"#<IPAddr: IPv6:0000:0000:0000:0000:0000:0000:c0a8:0100/0000:0000:0000:0000:0000:0000:ffff:ff00>"
0
false
no implicit conversion of Integer into String
