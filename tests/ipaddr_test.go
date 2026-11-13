package tests

import (
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
)

// The expected strings are Ruby's, with ipaddr 1.2.7, but for the error
// messages: Ruby 3.4.1's name no address, and cameo's name the address
// or the length that was refused.

func TestIPAddrFollowsRubys(t *testing.T) {
	out := runScript(t, `require "ipaddr"
a = IPAddr.new("192.168.1.10")
n = IPAddr.new("192.168.1.0/24")
v6 = IPAddr.new("2001:db8::1")
n6 = IPAddr.new("2001:db8::/32")
p [a, n, v6, n6], [a.to_s, n.to_s, v6.to_s, n6.to_s, a.to_string, v6.to_string, n6.to_string]
p [a.to_i, n.to_i, v6.to_i, a.prefix, n.prefix, v6.prefix, n6.prefix, a.family, v6.family, a.ipv4?, a.ipv6?, v6.ipv6?], [a.netmask, n.netmask, n6.netmask]
p [n.include?(a), n.include?(IPAddr.new("192.168.2.1")), n === "192.168.1.77", n6.include?(v6), n.include?(v6), a.include?(a), n.include?(IPAddr.new("192.168.1.0/25")), IPAddr.new("192.168.1.0/25").include?(n)]
p [a == IPAddr.new("192.168.1.10"), a == "192.168.1.10", a.eql?(IPAddr.new("192.168.1.10")), a == n, a <=> n, n <=> a, a <=> a, a <=> v6, a <=> 1, {a => 1}[IPAddr.new("192.168.1.10")], Set[a, IPAddr.new("192.168.1.10"), n].size]
p [a.mask(24), a.mask("255.255.0.0"), v6.mask(64), n.mask(16), IPAddr.new("1.2.3.4").mask("255.255.255.0/24")], n.to_range, IPAddr.new("10.0.0.0/30").to_range.map(&:to_s)
p [a.succ, IPAddr.new("192.168.1.255").succ.to_s, v6.succ.to_s, n.succ], (a..IPAddr.new("192.168.1.12")).map(&:to_s), [IPAddr.new("10.0.0.2"), IPAddr.new("10.0.0.1")].sort.map(&:to_s), a.between?(n, IPAddr.new("192.168.2.0"))
p [IPAddr.new("10.1.2.3").private?, IPAddr.new("172.16.0.1").private?, IPAddr.new("172.32.0.1").private?, IPAddr.new("192.168.0.1").private?, IPAddr.new("8.8.8.8").private?, IPAddr.new("fc00::1").private?, IPAddr.new("fe80::1").private?, IPAddr.new("127.0.0.1").loopback?, IPAddr.new("::1").loopback?, IPAddr.new("::2").loopback?, IPAddr.new("169.254.1.1").link_local?, IPAddr.new("fe80::1").link_local?, IPAddr.new("10.0.0.1").link_local?]
p [a | 255, (a & IPAddr.new("255.255.255.0")).to_s, (a << 8).to_s, (a >> 8).to_s, (~a).to_s, (a | "0.0.0.255").to_s], [a.reverse, v6.reverse, v6.ip6_int]
p [IPAddr.new("::ffff:192.168.1.1").ipv4_mapped?, IPAddr.new("::ffff:192.168.1.1").native.to_s, a.ipv4_mapped.to_s, a.ipv4_compat.to_s, IPAddr.new("::192.168.1.1").ipv4_compat?, v6.ipv4_mapped?, a.native.to_s, IPAddr.new("::ffff:192.168.1.1").to_s]
p [IPAddr.new("192.168.1.10/255.255.255.0").inspect, IPAddr.new("0.0.0.0/0").inspect, IPAddr.new("::/0").to_s, IPAddr.new(3232235786, Socket::AF_INET).to_s, IPAddr.new(1, Socket::AF_INET6).to_s, IPAddr.new.to_s, IPAddr.new.family]
p ["2001:0db8:0000:0000:0000:0000:0000:0001", "2001:db8:0:0:1:0:0:1", "::", "1::", "0:0:0:0:0:0:0:1", "::1:2", "64:ff9b::1.2.3.4"].map { |s| IPAddr.new(s).to_s }, IPAddr.new("fe80::1%eth0").zone_id, IPAddr.new("fe80::1%eth0").to_s, a.hton.bytes, IPAddr.ntop(b"\xC0\xA8\x01\x0A")
["256.1.1.1", "1.2.3", "abc", "", "1.2.3.4/33", "1.2.3.4/-1", "1.2.3.4/255.255.0.255", "::1/129", "2001:db8::g", "01.2.3.4", "1.2.3.4/24/", "1.2.3.4/::"].each do |s|
  IPAddr.new(s)
rescue IPAddr::Error => e
  puts "#{e.class}: #{e.message}"
end
begin; IPAddr.new(1); rescue IPAddr::Error => e; puts "#{e.class}: #{e.message}"; end
begin; a.ip6_arpa; rescue IPAddr::Error => e; puts "#{e.class}: #{e.message}"; end
p IPAddr::InvalidAddressError.ancestors.take(3), IPAddr::InvalidPrefixError.superclass, IPAddr::VERSION, IPAddr::IN4MASK, IPAddr::IN6MASK.to_s(16)
`)
	assert.Equals(t, out, `[#<IPAddr: IPv4:192.168.1.10/255.255.255.255>, #<IPAddr: IPv4:192.168.1.0/255.255.255.0>, #<IPAddr: IPv6:2001:0db8:0000:0000:0000:0000:0000:0001/ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff>, #<IPAddr: IPv6:2001:0db8:0000:0000:0000:0000:0000:0000/ffff:ffff:0000:0000:0000:0000:0000:0000>]
["192.168.1.10", "192.168.1.0", "2001:db8::1", "2001:db8::", "192.168.1.10", "2001:0db8:0000:0000:0000:0000:0000:0001", "2001:0db8:0000:0000:0000:0000:0000:0000"]
[3232235786, 3232235776, 42540766411282592856903984951653826561, 32, 24, 128, 32, 2, 10, true, false, true]
["255.255.255.255", "255.255.255.0", "ffff:ffff:0000:0000:0000:0000:0000:0000"]
[true, false, true, true, false, true, true, false]
[true, true, true, false, 1, -1, 0, nil, 1, 1, 2]
[#<IPAddr: IPv4:192.168.1.0/255.255.255.0>, #<IPAddr: IPv4:192.168.0.0/255.255.0.0>, #<IPAddr: IPv6:2001:0db8:0000:0000:0000:0000:0000:0000/ffff:ffff:ffff:ffff:0000:0000:0000:0000>, #<IPAddr: IPv4:192.168.0.0/255.255.0.0>, #<IPAddr: IPv4:1.2.3.0/255.255.255.0>]
#<IPAddr: IPv4:192.168.1.0/255.255.255.255>..#<IPAddr: IPv4:192.168.1.255/255.255.255.255>
["10.0.0.0", "10.0.0.1", "10.0.0.2", "10.0.0.3"]
[#<IPAddr: IPv4:192.168.1.11/255.255.255.255>, "192.168.2.0", "2001:db8::2", #<IPAddr: IPv4:192.168.1.1/255.255.255.0>]
["192.168.1.10", "192.168.1.11", "192.168.1.12"]
["10.0.0.1", "10.0.0.2"]
true
[true, true, false, true, false, true, false, true, true, false, true, true, false]
[#<IPAddr: IPv4:192.168.1.255/255.255.255.255>, "192.168.1.0", "168.1.10.0", "0.192.168.1", "63.87.254.245", "192.168.1.255"]
["10.1.168.192.in-addr.arpa", "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa", "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.int"]
[true, "192.168.1.1", "::ffff:192.168.1.10", "::192.168.1.10", true, false, "192.168.1.10", "::ffff:192.168.1.1"]
["#<IPAddr: IPv4:192.168.1.0/255.255.255.0>", "#<IPAddr: IPv4:0.0.0.0/0.0.0.0>", "::", "192.168.1.10", "::1", "::", 10]
["2001:db8::1", "2001:db8::1:0:0:1", "::", "1::", "::1", "::0.1.0.2", "64:ff9b::102:304"]
"%eth0"
"fe80::1%eth0"
[192, 168, 1, 10]
"192.168.1.10"
IPAddr::InvalidAddressError: invalid address: 256.1.1.1
IPAddr::InvalidAddressError: invalid address: 1.2.3
IPAddr::InvalidAddressError: invalid address: abc
IPAddr::InvalidAddressError: invalid address: 
IPAddr::InvalidPrefixError: invalid length: 33
IPAddr::InvalidAddressError: invalid address: -1
IPAddr::InvalidPrefixError: invalid mask 255.255.0.255
IPAddr::InvalidPrefixError: invalid length: 129
IPAddr::InvalidAddressError: invalid address: 2001:db8::g
IPAddr::InvalidAddressError: zero-filled number in IPv4 address is ambiguous: 01.2.3.4
IPAddr::InvalidAddressError: invalid address: 24
IPAddr::InvalidPrefixError: address family is not same
IPAddr::AddressFamilyError: address family must be specified
IPAddr::InvalidAddressError: not an IPv6 address: 3232235786
[IPAddr::InvalidAddressError, IPAddr::Error, ArgumentError]
IPAddr::InvalidAddressError
"1.2.7"
4294967295
"ffffffffffffffffffffffffffffffff"
`)
}

// Shifts, comparisons, inclusion, shortening, prefixes, zones and
// overflow follow ipaddr 1.2.7: a negative shift count shifts the other
// way, == and <=> take any Integer, include? and to_range use the
// masked address, ::ffff:1 is ::255.255.0.1, a prefix with leading zeros
// is refused, a zone id is word characters and part of eql?, an address
// past the family is refused rather than wrapped, and ipv4_compat keeps
// the IPv4 mask. The messages that name an address are cameo's.
func TestIPAddrShiftsComparesAndRefusesAsRubyDoes(t *testing.T) {
	out := runScript(t, `require "ipaddr"
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
`)
	assert.Equals(t, out, `"96.84.0.133"
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
`)
}
