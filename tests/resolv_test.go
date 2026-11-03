package tests

import (
	"bytes"
	"net"
	"testing"

	"github.com/cameo-lang/cameo/src/assert"
	"github.com/cameo-lang/cameo/src/cameo"
)

// The expected values are Ruby 3.4's, with resolv 0.6.0.

// runResolving runs a script with the system's resolver, as the cameo
// command does.
func runResolving(t *testing.T, src string) string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	prog, err := cameo.Load("t.cameo", src, &stdout, &stderr)
	if err != nil {
		t.Fatalf("%s\nfailed to load: %v", src, err)
	}
	prog.Resolver(net.DefaultResolver)
	if err := prog.Run(); err != nil {
		t.Fatalf("%s\nfailed: %v\nstderr: %s", src, err, stderr.String())
	}
	return stdout.String()
}

func TestResolvAddressesNamesAndHosts(t *testing.T) {
	out := runResolving(t, `require "resolv"
p Resolv.getaddress("127.0.0.1"), Resolv.getaddresses("::1"), Resolv.getaddress("localhost"), Resolv.getaddresses("localhost").include?("127.0.0.1")
p Resolv.getname("127.0.0.1"), Resolv.getnames("127.0.0.1").include?("localhost")
r = []
Resolv.each_address("127.0.0.1") { |a| r += [a] }
p r
h = Resolv::Hosts.new
p h.getaddress("localhost"), h.getaddresses("localhost").include?("127.0.0.1"), h.getname("127.0.0.1")
dns = Resolv::DNS.new
p dns.getresources("127.0.0.1", Resolv::DNS::Resource::IN::MX), Resolv::DNS.open { |d| d.class }
[-> { Resolv.getaddress("no-such-host.invalid") }, -> { Resolv.getname("192.0.2.1") }, -> { Resolv::Hosts.new.getaddress("no-such-host.invalid") },
 -> { Resolv::DNS.new.getaddress("no-such-host.invalid") }, -> { Resolv::DNS.new.getresource("no-such-host.invalid", Resolv::DNS::Resource::IN::A) },
 -> { Resolv::DNS.new.getresources("no-such-host.invalid", Resolv::DNS::Resource::IN::A) }].each do |f|
  begin
    p f.call
  rescue Resolv::ResolvError => e
    puts "#{e.class}: #{e.message}"
  end
end
`)
	assert.Equals(t, out, "\"127.0.0.1\"\n[\"::1\"]\n\"127.0.0.1\"\ntrue\n\"localhost\"\ntrue\n[\"127.0.0.1\"]\n\"127.0.0.1\"\ntrue\n\"localhost\"\n[]\nResolv::DNS\nResolv::ResolvError: no address for no-such-host.invalid\nResolv::ResolvError: no name for 192.0.2.1\nResolv::ResolvError: /etc/hosts has no name: no-such-host.invalid\nResolv::ResolvError: DNS result has no information for no-such-host.invalid\nResolv::ResolvError: DNS result has no information for no-such-host.invalid\n[]\n")
}

func TestResolvWithoutAResolverRaises(t *testing.T) {
	out := runScript(t, `require "resolv"
p Resolv.getaddress("127.0.0.1"), Resolv.getaddress("localhost"), Resolv::Hosts.new.getname("127.0.0.1")
begin
  Resolv.getaddress("example.com")
rescue Resolv::ResolvError => e
  puts e.message
end
begin
  Resolv::DNS.new.getresources("example.com", Resolv::DNS::Resource::IN::MX)
rescue Resolv::ResolvError => e
  puts e.message
end
`)
	assert.Equals(t, out, "\"127.0.0.1\"\n\"127.0.0.1\"\n\"localhost\"\nno address for example.com (the program has no network access: its host gave it no resolver)\nDNS result has no information for example.com (the program has no network access: its host gave it no resolver)\n")
}

func TestResolvValuesMatchRuby(t *testing.T) {
	out := runScript(t, `require "resolv"
p Resolv::ResolvError.superclass, Resolv::ResolvTimeout.superclass, Resolv::DNS::Resource::IN::A.superclass, Resolv::DNS::Resource::IN::MX.superclass, Resolv::IPv4::Regex.class, Resolv::DNS::Port, Resolv::DNS::UDPSize
p Resolv::IPv4.create("10.0.0.1").to_s, Resolv::IPv4.create("10.0.0.1").address, Resolv::IPv4.create("10.0.0.1").to_name.to_s, Resolv::IPv4.create("10.0.0.1") == Resolv::IPv4.create("10.0.0.1"), Resolv::IPv4::Regex =~ "1.2.3.4", Resolv::IPv4::Regex =~ "1.2.3", Resolv::IPv4::Regex =~ "256.1.1.1", Resolv::IPv4.create("1.2.3.4").inspect
p Resolv::IPv6.create("::1").to_s, Resolv::IPv6.create("2001:db8::1").to_s, Resolv::IPv6.create("2001:DB8:0:0:0:0:0:1").to_s, Resolv::IPv6.create("::ffff:1.2.3.4").to_s, Resolv::IPv6.create("::1").to_name.to_s, Resolv::IPv6::Regex =~ "::1", Resolv::IPv6::Regex =~ "1.2.3.4", Resolv::IPv6.create("::1").inspect, Resolv::IPv6.create("::1").address.bytesize
n = Resolv::DNS::Name.create("www.example.com")
p n.to_s, n.absolute?, Resolv::DNS::Name.create("www.example.com.").absolute?, n.to_a, n.subdomain_of?(Resolv::DNS::Name.create("example.com")), n == Resolv::DNS::Name.create("www.example.com"), n.inspect, n.length, n[0], Resolv::DNS::Name.create("www.example.com.").to_s
[-> { Resolv::IPv4.create("1.2.3") }, -> { Resolv::IPv6.create("zz") }].each do |f|
  begin
    f.call
  rescue ArgumentError => e
    puts e.message
  end
end
`)
	assert.Equals(t, out, "StandardError\nTimeout::Error\nResolv::DNS::Resource\nResolv::DNS::Resource::MX\nRegexp\n53\n512\n\"10.0.0.1\"\n\"\\n\\x00\\x00\\x01\"\n\"1.0.0.10.in-addr.arpa\"\ntrue\n0\nnil\nnil\n\"#<Resolv::IPv4 1.2.3.4>\"\n\"::1\"\n\"2001:db8::1\"\n\"2001:db8::1\"\n\"::ffff:102:304\"\n\"1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.ip6.arpa\"\n0\nnil\n\"#<Resolv::IPv6 ::1>\"\n16\n\"www.example.com\"\nfalse\ntrue\n[\"www\", \"example\", \"com\"]\ntrue\ntrue\n\"#<Resolv::DNS::Name: www.example.com>\"\n3\n\"www\"\n\"www.example.com\"\ncannot interpret as IPv4 address: \"1.2.3\"\nnot numeric IPv6 address: zz\n")
}
