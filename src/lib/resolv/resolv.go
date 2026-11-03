// Package resolv is Ruby's resolv library, built into cameo: Resolv,
// Resolv::Hosts, Resolv::DNS with its resources, Resolv::IPv4, IPv6 and
// Resolv::DNS::Name.
//
// It follows resolv 0.6.0, the version Ruby 3.4 ships. The lookups go
// through the resolver the host gave the program, Go's net.Resolver, as
// net/http's requests go through its client: the cameo command gives the
// system's, an embedding program decides. Without one, a lookup raises
// Resolv::ResolvError. Go's resolver answers A, AAAA, PTR, MX, TXT, CNAME,
// NS and SRV queries and gives no time to live, so a resource's ttl is
// nil; SOA and the other types are left out.
package resolv

import (
	"context"
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/protogen"
	"github.com/cameo-lang/cameo/src/stdlib"
)

var (
	resolvClass  = object.NewClass("Resolv", object.ObjectClass)
	resolvError  = object.NewClass("ResolvError", object.StandardError)
	resolvTimout *object.Class
	hostsClass   = object.NewClass("Hosts", object.ObjectClass)
	dnsClass     = object.NewClass("DNS", object.ObjectClass)
	ipv4Class    = object.NewClass("IPv4", object.ObjectClass)
	ipv6Class    = object.NewClass("IPv6", object.ObjectClass)
	nameClass    = object.NewClass("Name", object.ObjectClass)
	resourceMod  = object.NewClass("Resource", object.ObjectClass)
	inMod        = object.NewModule("IN")
	resources    = map[string]*object.Class{}
	queryMod     = object.NewClass("Query", object.ObjectClass)
)

// host is what the interpreter gives the library: the resolver the Go
// host set. eval.Interp implements it.
type host interface {
	Resolver() *net.Resolver
}

func resolverOf(ctx object.Context) *net.Resolver {
	if h, ok := ctx.(host); ok {
		return h.Resolver()
	}
	return nil
}

func fail(format string, args ...any) error {
	return &object.Exception{Cls: resolvError, Message: fmt.Sprintf(format, args...)}
}

func noResolver(what string) error {
	return fail("%s (the program has no network access: its host gave it no resolver)", what)
}

// Values: an IPv4, an IPv6 and a Name are values with the text as their
// one field; a resource has its fields.

func declare(cls *object.Class, fields ...string) {
	used := map[int64]bool{}
	taken := func(n int64) bool {
		if used[n] {
			return true
		}
		used[n] = true
		return false
	}
	for _, name := range fields {
		typ := object.ScalarField("string")
		if name == "ttl" || name == "preference" || name == "priority" || name == "weight" || name == "port" {
			typ = object.ScalarField("int")
		}
		f := &object.Field{Name: name, Number: protogen.FieldNumber(name, taken), Type: typ, Owner: cls}
		cls.FieldIndex[name] = len(cls.Fields)
		cls.Fields = append(cls.Fields, f)
	}
}

func instance(cls *object.Class, fields ...object.Value) object.Value {
	return object.InstanceValue(&object.Instance{Cls: cls, Fields: fields})
}

func text(v object.Value) string { return v.Instance().Fields[0].Str() }

func newIPv4(s string) object.Value { return instance(ipv4Class, object.String(s)) }
func newIPv6(s string) object.Value { return instance(ipv6Class, object.String(s)) }
func newName(s string) object.Value { return instance(nameClass, object.String(s)) }

// addressValue is the IPv4 or IPv6 of a parsed IP.
func addressValue(ip net.IP) object.Value {
	if ip4 := ip.To4(); ip4 != nil {
		return newIPv4(ip4.String())
	}
	return newIPv6(ipv6String(ip))
}

// ipv6String compresses an IPv6 address as Ruby does: hex groups all the
// way, ::ffff:102:304 for a mapped IPv4 address.
func ipv6String(ip net.IP) string {
	ip = ip.To16()
	groups := make([]uint16, 8)
	for i := range groups {
		groups[i] = uint16(ip[2*i])<<8 | uint16(ip[2*i+1])
	}
	// The longest run of zero groups becomes ::.
	bestStart, bestLen := -1, 0
	for i := 0; i < 8; {
		if groups[i] != 0 {
			i++
			continue
		}
		j := i
		for j < 8 && groups[j] == 0 {
			j++
		}
		if j-i > bestLen && j-i >= 2 {
			bestStart, bestLen = i, j-i
		}
		i = j
	}
	var parts []string
	for i := 0; i < 8; i++ {
		if i == bestStart {
			parts = append(parts, "")
			if i == 0 {
				parts = append(parts, "")
			}
			i += bestLen - 1
			if i == 7 {
				parts = append(parts, "")
			}
			continue
		}
		parts = append(parts, strconv.FormatUint(uint64(groups[i]), 16))
	}
	return strings.Join(parts, ":")
}

// parseIPv4 reads a dotted quad, strictly.
func parseIPv4(s string) (net.IP, bool) {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return nil, false
	}
	ip := make(net.IP, 4)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 || p == "" || len(p) > 1 && p[0] == '0' && false {
			return nil, false
		}
		ip[i] = byte(n)
	}
	return ip, true
}

func parseIPv6(s string) (net.IP, bool) {
	if !strings.Contains(s, ":") {
		return nil, false
	}
	ip := net.ParseIP(s)
	if ip == nil {
		return nil, false
	}
	return ip.To16(), true
}

// reverseName is the PTR name of an address.
func reverseName(ip net.IP) string {
	if ip4 := ip.To4(); ip4 != nil {
		return fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa", ip4[3], ip4[2], ip4[1], ip4[0])
	}
	ip = ip.To16()
	var b strings.Builder
	for i := 15; i >= 0; i-- {
		b.WriteString(fmt.Sprintf("%x.%x.", ip[i]&0xf, ip[i]>>4))
	}
	return b.String() + "ip6.arpa"
}

// Lookups.

// lookupHosts reads /etc/hosts: the addresses of a name, or the names of
// an address.
func lookupHosts(path string) (addrs map[string][]string, names map[string][]string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	addrs, names = map[string][]string{}, map[string][]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip := net.ParseIP(fields[0])
		if ip == nil {
			continue
		}
		addr := ip.String()
		for _, name := range fields[1:] {
			key := strings.ToLower(name)
			addrs[key] = append(addrs[key], addr)
			names[addr] = append(names[addr], name)
		}
	}
	return addrs, names, nil
}

// addressesOf resolves a name to its addresses: an address stands for
// itself, /etc/hosts answers first, then the resolver.
func addressesOf(ctx object.Context, r *net.Resolver, name string, timeout time.Duration) ([]string, error) {
	if ip := net.ParseIP(name); ip != nil {
		return []string{ip.String()}, nil
	}
	if addrs, _, err := lookupHosts("/etc/hosts"); err == nil {
		if found := addrs[strings.ToLower(name)]; len(found) > 0 {
			return found, nil
		}
	}
	if r == nil {
		return nil, nil
	}
	c, cancel := withTimeout(timeout)
	defer cancel()
	ips, err := r.LookupIPAddr(c, name)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(ips))
	for i, ip := range ips {
		out[i] = ip.IP.String()
	}
	return out, nil
}

func namesOf(ctx object.Context, r *net.Resolver, addr string, timeout time.Duration) ([]string, error) {
	if _, names, err := lookupHosts("/etc/hosts"); err == nil {
		if ip := net.ParseIP(addr); ip != nil {
			if found := names[ip.String()]; len(found) > 0 {
				return found, nil
			}
		}
	}
	if r == nil {
		return nil, nil
	}
	c, cancel := withTimeout(timeout)
	defer cancel()
	names, err := r.LookupAddr(c, addr)
	if err != nil {
		return nil, err
	}
	for i, n := range names {
		names[i] = strings.TrimSuffix(n, ".")
	}
	return names, nil
}

func withTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	if d <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), d)
}

func isTimeout(err error) bool {
	if ne, ok := err.(net.Error); ok {
		return ne.Timeout()
	}
	return err == context.DeadlineExceeded
}

// dns is a Resolv::DNS: its settings, a handle.
type dns struct {
	resolver *net.Resolver
	timeout  time.Duration
	servers  []string
}

func dnsOf(v object.Value) *dns { return v.Instance().Message.(*dns) }

// Install defines Resolv. lib.Install calls it once.
func Install() {
	r := resolvClass
	object.ObjectClass.Consts["Resolv"] = object.ClassValue(r)
	timeoutMod, ok := object.ObjectClass.Consts["Timeout"]
	var timeoutError *object.Class
	if ok {
		tm, _ := timeoutMod.AsClass()
		if v, ok := tm.Consts["Error"]; ok {
			timeoutError, _ = v.AsClass()
		}
	}
	if timeoutError == nil {
		tm := object.NewModule("Timeout")
		timeoutError = object.NewClass("Error", object.RuntimeError)
		timeoutError.Lexical = tm
		tm.Consts["Error"] = object.ClassValue(timeoutError)
		object.ObjectClass.Consts["Timeout"] = object.ClassValue(tm)
	}
	resolvTimout = object.NewClass("ResolvTimeout", timeoutError)
	for _, c := range []*object.Class{resolvError, resolvTimout, hostsClass, dnsClass, ipv4Class, ipv6Class} {
		c.Lexical = r
		r.Consts[c.Name] = object.ClassValue(c)
	}
	nameClass.Lexical, resourceMod.Lexical, queryMod.Lexical = dnsClass, dnsClass, dnsClass
	dnsClass.Consts["Name"] = object.ClassValue(nameClass)
	dnsClass.Consts["Resource"] = object.ClassValue(resourceMod)
	dnsClass.Consts["Query"] = object.ClassValue(queryMod)
	dnsClass.Consts["Port"] = object.Int(53)
	dnsClass.Consts["UDPSize"] = object.Int(512)
	inMod.Lexical = resourceMod
	resourceMod.Consts["IN"] = object.ClassValue(inMod)
	declare(ipv4Class, "address")
	declare(ipv6Class, "address")
	declare(nameClass, "name")
	for _, t := range []struct {
		name   string
		fields []string
	}{
		{"A", []string{"address"}}, {"AAAA", []string{"address"}}, {"MX", []string{"preference", "exchange"}},
		{"TXT", []string{"strings"}}, {"CNAME", []string{"name"}}, {"NS", []string{"name"}}, {"PTR", []string{"name"}},
		{"SRV", []string{"priority", "weight", "port", "target"}}, {"SOA", []string{"mname", "rname", "serial", "refresh", "retry", "expire", "minimum"}}, {"ANY", nil},
	} {
		// Resource::MX is the generic record, Resource::IN::MX the one of
		// the Internet class, as in Ruby; A and AAAA exist only under IN.
		parent := resourceMod
		if t.name != "A" && t.name != "AAAA" {
			generic := object.NewClass(t.name, resourceMod)
			generic.Lexical = resourceMod
			resourceMod.Consts[t.name] = object.ClassValue(generic)
			parent = generic
		}
		cls := object.NewClass(t.name, parent)
		cls.Lexical = inMod
		inMod.Consts[t.name] = object.ClassValue(cls)
		declare(cls, t.fields...)
		resources[t.name] = cls
	}
	installResources()
	installAddresses()
	installName()
	installResolv()
	installHosts()
	installDNS()
}

func kindName(v object.Value) string { return stdlib.KindName(v) }

// resourceType is the type a Resource class argument names.
func resourceType(ctx object.Context, v object.Value) (string, error) {
	cls, ok := v.AsClass()
	if !ok {
		return "", ctx.Raise(object.TypeError, "a resource class, not %s", kindName(v))
	}
	for name, c := range resources {
		if cls == c || cls == c.Super {
			return name, nil
		}
	}
	return "", ctx.Raise(object.ArgumentError, "unsupported resource class %s", cls.QualifiedName())
}

func installResources() {
	reader := func(cls *object.Class, name string, index int, wrap func(object.Value) object.Value) {
		cls.Define(name, 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			v := self.Instance().Fields[index]
			if wrap != nil {
				return wrap(v), nil
			}
			return v, nil
		})
	}
	asName := func(v object.Value) object.Value { return newName(v.Str()) }
	for name, cls := range resources {
		cls.Define("ttl", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Nil, nil
		})
		switch name {
		case "A":
			reader(cls, "address", 0, func(v object.Value) object.Value { return newIPv4(v.Str()) })
		case "AAAA":
			reader(cls, "address", 0, func(v object.Value) object.Value { return newIPv6(v.Str()) })
		case "MX":
			reader(cls, "preference", 0, nil)
			reader(cls, "exchange", 1, asName)
		case "TXT":
			cls.Define("strings", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
				parts := strings.Split(self.Instance().Fields[0].Str(), "\x00")
				out := make([]object.Value, len(parts))
				for i, p := range parts {
					out[i] = object.String(p)
				}
				return object.NewList(out...), nil
			})
			cls.Define("data", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
				return object.String(strings.ReplaceAll(self.Instance().Fields[0].Str(), "\x00", "")), nil
			})
		case "CNAME", "NS", "PTR":
			reader(cls, "name", 0, asName)
		case "SRV":
			reader(cls, "priority", 0, nil)
			reader(cls, "weight", 1, nil)
			reader(cls, "port", 2, nil)
			reader(cls, "target", 3, asName)
		}
	}
}

func installAddresses() {
	for _, c := range []*object.Class{ipv4Class, ipv6Class} {
		cls := c
		v6 := cls == ipv6Class
		cls.DefineSingleton("create", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			if inst, ok := args[0].AsInstance(); ok && inst.Cls == cls {
				return args[0], nil
			}
			s, err := stdlib.WantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			if v6 {
				ip, ok := parseIPv6(s)
				if !ok {
					return object.Value{}, ctx.Raise(object.ArgumentError, "not numeric IPv6 address: %s", s)
				}
				return newIPv6(ipv6String(ip)), nil
			}
			ip, ok := parseIPv4(s)
			if !ok {
				return object.Value{}, ctx.Raise(object.ArgumentError, "cannot interpret as IPv4 address: %s", strconv.Quote(s))
			}
			return newIPv4(ip.String()), nil
		})
		cls.Define("to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.String(text(self)), nil
		})
		cls.Define("inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.String("#<" + cls.QualifiedName() + " " + text(self) + ">"), nil
		})
		cls.Define("address", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			ip := net.ParseIP(text(self))
			if !v6 {
				ip = ip.To4()
			}
			return object.Bytes([]byte(ip.To16()[16-len(ip):])), nil
		})
		cls.Define("to_name", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return newName(reverseName(net.ParseIP(text(self)))), nil
		})
		cls.Define("==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			inst, ok := args[0].AsInstance()
			return object.Bool(ok && inst.Cls == cls && text(args[0]) == text(self)), nil
		})
		cls.Define("hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return object.Int(int64(len(text(self)))), nil
		})
	}
	v4re, _ := object.NewRegexp(`\A((?x-mi:0|1(?:[0-9][0-9]?)?|2(?:[0-4][0-9]?|5[0-5]?|[6-9])?|[3-9][0-9]?))\.((?x-mi:0|1(?:[0-9][0-9]?)?|2(?:[0-4][0-9]?|5[0-5]?|[6-9])?|[3-9][0-9]?))\.((?x-mi:0|1(?:[0-9][0-9]?)?|2(?:[0-4][0-9]?|5[0-5]?|[6-9])?|[3-9][0-9]?))\.((?x-mi:0|1(?:[0-9][0-9]?)?|2(?:[0-4][0-9]?|5[0-5]?|[6-9])?|[3-9][0-9]?))\z`, 0)
	if v4re != nil {
		ipv4Class.Consts["Regex"] = object.RegexpValue(v4re)
	}
	v6re, _ := object.NewRegexp(`\A(?:(?:[0-9A-Fa-f]{1,4}:){7}[0-9A-Fa-f]{1,4}|(?:[0-9A-Fa-f]{1,4}(?::[0-9A-Fa-f]{1,4})*)?::(?:[0-9A-Fa-f]{1,4}(?::[0-9A-Fa-f]{1,4})*)?|(?:[0-9A-Fa-f]{1,4}:){6}\d+\.\d+\.\d+\.\d+|(?:[0-9A-Fa-f]{1,4}(?::[0-9A-Fa-f]{1,4})*)?::(?:[0-9A-Fa-f]{1,4}:)*\d+\.\d+\.\d+\.\d+)\z`, 0)
	if v6re != nil {
		ipv6Class.Consts["Regex"] = object.RegexpValue(v6re)
	}
}

func installName() {
	c := nameClass
	c.DefineSingleton("create", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if inst, ok := args[0].AsInstance(); ok && inst.Cls == c {
			return args[0], nil
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return newName(s), nil
	})
	labels := func(self object.Value) []string {
		s := strings.TrimSuffix(text(self), ".")
		if s == "" {
			return nil
		}
		return strings.Split(s, ".")
	}
	c.Define("to_s", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String(strings.TrimSuffix(text(self), ".")), nil
	})
	c.Define("inspect", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.String("#<Resolv::DNS::Name: " + text(self) + ">"), nil
	})
	c.Define("absolute?", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Bool(strings.HasSuffix(text(self), ".")), nil
	})
	c.Define("to_a", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		ls := labels(self)
		out := make([]object.Value, len(ls))
		for i, l := range ls {
			out[i] = object.String(l)
		}
		return object.NewList(out...), nil
	})
	c.Define("length", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(len(labels(self)))), nil
	})
	c.Define("[]", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		i, err := stdlib.WantInt(ctx, args[0], "")
		if err != nil {
			return object.Value{}, err
		}
		ls := labels(self)
		if i < 0 {
			i += int64(len(ls))
		}
		if i < 0 || i >= int64(len(ls)) {
			return object.Nil, nil
		}
		return object.String(ls[i]), nil
	})
	c.Define("subdomain_of?", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		var other string
		if inst, ok := args[0].AsInstance(); ok && inst.Cls == c {
			other = text(args[0])
		} else {
			var err error
			if other, err = stdlib.WantString(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		mine, theirs := strings.ToLower(strings.TrimSuffix(text(self), ".")), strings.ToLower(strings.TrimSuffix(other, "."))
		return object.Bool(len(mine) > len(theirs) && strings.HasSuffix(mine, "."+theirs)), nil
	})
	c.Define("==", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		inst, ok := args[0].AsInstance()
		return object.Bool(ok && inst.Cls == c && strings.EqualFold(text(args[0]), text(self))), nil
	})
	c.Define("hash", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Int(int64(len(text(self)))), nil
	})
}

func stringsValue(ss []string) object.Value {
	out := make([]object.Value, len(ss))
	for i, s := range ss {
		out[i] = object.String(s)
	}
	return object.NewList(out...)
}

// lookupMethods defines getaddress, getaddresses, each_address, getname,
// getnames and each_name on a class, over the given lookups.
func lookupMethods(cls *object.Class, singleton bool, addresses func(ctx object.Context, self object.Value, name string) ([]string, error), names func(ctx object.Context, self object.Value, addr string) ([]string, error), noAddress, noName func(string) error) {
	define := func(name string, arity int, f object.NativeFn) {
		if singleton {
			cls.DefineSingleton(name, arity, f)
		} else {
			cls.Define(name, arity, f)
		}
	}
	define("getaddress", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		addrs, err := addresses(ctx, self, name)
		if err != nil {
			return object.Value{}, err
		}
		if len(addrs) == 0 {
			return object.Value{}, noAddress(name)
		}
		return object.String(addrs[0]), nil
	})
	define("getaddresses", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		addrs, err := addresses(ctx, self, name)
		if err != nil {
			return object.Value{}, err
		}
		return stringsValue(addrs), nil
	})
	define("each_address", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		name, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		addrs, err := addresses(ctx, self, name)
		if err != nil {
			return object.Value{}, err
		}
		for _, a := range addrs {
			if _, err := ctx.Yield(block, object.String(a)); err != nil {
				return object.Value{}, err
			}
		}
		return object.Nil, nil
	})
	define("getname", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		addr, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		found, err := names(ctx, self, addr)
		if err != nil {
			return object.Value{}, err
		}
		if len(found) == 0 {
			return object.Value{}, noName(addr)
		}
		return object.String(found[0]), nil
	})
	define("getnames", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		addr, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		found, err := names(ctx, self, addr)
		if err != nil {
			return object.Value{}, err
		}
		return stringsValue(found), nil
	})
	define("each_name", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		addr, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		found, err := names(ctx, self, addr)
		if err != nil {
			return object.Value{}, err
		}
		for _, n := range found {
			if _, err := ctx.Yield(block, object.String(n)); err != nil {
				return object.Value{}, err
			}
		}
		return object.Nil, nil
	})
}

// netError turns a resolver's error into Resolv's: a timeout, or nothing
// found, which the callers report by name.
func netError(err error) ([]string, error) {
	if err == nil {
		return nil, nil
	}
	if isTimeout(err) {
		// Ruby's lookups report a timeout as no information.
		return nil, nil
	}
	if dnsErr, ok := err.(*net.DNSError); ok && (dnsErr.IsNotFound || strings.Contains(dnsErr.Err, "no such host")) {
		return nil, nil
	}
	return nil, fail("%s", err.Error())
}

func installResolv() {
	lookupMethods(resolvClass, true,
		func(ctx object.Context, self object.Value, name string) ([]string, error) {
			addrs, err := addressesOf(ctx, resolverOf(ctx), name, 0)
			if err != nil {
				return netError(err)
			}
			if len(addrs) == 0 && resolverOf(ctx) == nil {
				return nil, noResolver("no address for " + name)
			}
			return addrs, nil
		},
		func(ctx object.Context, self object.Value, addr string) ([]string, error) {
			names, err := namesOf(ctx, resolverOf(ctx), addr, 0)
			if err != nil {
				return netError(err)
			}
			if len(names) == 0 && resolverOf(ctx) == nil {
				return nil, noResolver("no name for " + addr)
			}
			return names, nil
		},
		func(name string) error { return fail("no address for %s", name) },
		func(addr string) error { return fail("no name for %s", addr) })
}

func installHosts() {
	c := hostsClass
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		path := "/etc/hosts"
		if len(args) > 0 {
			var err error
			if path, err = stdlib.WantString(ctx, args[0]); err != nil {
				return object.Value{}, err
			}
		}
		return object.InstanceValue(&object.Instance{Cls: c, Message: path}), nil
	})
	pathOf := func(v object.Value) string { return v.Instance().Message.(string) }
	lookupMethods(c, false,
		func(ctx object.Context, self object.Value, name string) ([]string, error) {
			addrs, _, err := lookupHosts(pathOf(self))
			if err != nil {
				return nil, fail("%s", err.Error())
			}
			return addrs[strings.ToLower(name)], nil
		},
		func(ctx object.Context, self object.Value, addr string) ([]string, error) {
			_, names, err := lookupHosts(pathOf(self))
			if err != nil {
				return nil, fail("%s", err.Error())
			}
			if ip := net.ParseIP(addr); ip != nil {
				return names[ip.String()], nil
			}
			return nil, nil
		},
		func(name string) error { return fail("/etc/hosts has no name: %s", name) },
		func(addr string) error { return fail("/etc/hosts has no address: %s", addr) })
}

func installDNS() {
	c := dnsClass
	newDNS := func(ctx object.Context, args []object.Value, kw *object.Map) (object.Value, error) {
		d := &dns{resolver: resolverOf(ctx)}
		config := kw
		if len(args) > 0 {
			if m, ok := args[0].AsMap(); ok {
				config = m
			}
		}
		if config != nil {
			if v, ok := config.Get(object.Symbol("nameserver")); ok {
				list, err := stdlib.ToList(ctx, v)
				if err != nil {
					list = []object.Value{v}
				}
				for _, s := range list {
					ns, err := stdlib.WantString(ctx, s)
					if err != nil {
						return object.Value{}, err
					}
					d.servers = append(d.servers, net.JoinHostPort(ns, "53"))
				}
			}
			if v, ok := config.Get(object.Symbol("nameserver_port")); ok {
				list, err := stdlib.ToList(ctx, v)
				if err != nil {
					return object.Value{}, err
				}
				for _, pair := range list {
					hp, err := stdlib.ToList(ctx, pair)
					if err != nil || len(hp) != 2 {
						return object.Value{}, ctx.Raise(object.ArgumentError, "nameserver_port takes [host, port] pairs")
					}
					h, err := stdlib.WantString(ctx, hp[0])
					if err != nil {
						return object.Value{}, err
					}
					p, err := stdlib.WantInt(ctx, hp[1], "")
					if err != nil {
						return object.Value{}, err
					}
					d.servers = append(d.servers, net.JoinHostPort(h, strconv.FormatInt(p, 10)))
				}
			}
		}
		if len(d.servers) > 0 && d.resolver != nil {
			servers := d.servers
			d.resolver = &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				var dialer net.Dialer
				var lastErr error
				for _, s := range servers {
					conn, err := dialer.DialContext(ctx, network, s)
					if err == nil {
						return conn, nil
					}
					lastErr = err
				}
				return nil, lastErr
			}}
		}
		return object.InstanceValue(&object.Instance{Cls: c, Message: d}), nil
	}
	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return newDNS(ctx, args, kw)
	})
	c.DefineSingleton("open", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		d, err := newDNS(ctx, args, kw)
		if err != nil || block.IsZero() {
			return d, err
		}
		return ctx.Yield(block, d)
	})
	c.Define("timeouts=", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		d := dnsOf(self)
		v := args[0]
		if list, ok := v.AsList(); ok && len(list.Elems) > 0 {
			v = list.Elems[0]
		}
		if v.IsNil() {
			d.timeout = 0
			return self, nil
		}
		f, ok := v.AsNumber()
		if !ok {
			return object.Value{}, ctx.Raise(object.TypeError, "timeouts take a number of seconds, not %s", kindName(v))
		}
		d.timeout = time.Duration(f * float64(time.Second))
		return self, nil
	})
	c.Define("timeouts", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		d := dnsOf(self)
		if d.timeout == 0 {
			return object.Nil, nil
		}
		return object.Float(d.timeout.Seconds()), nil
	})
	c.Define("close", 0, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		return object.Nil, nil
	})
	noInfo := func(name string) error { return fail("DNS result has no information for %s", name) }
	lookupMethods(c, false,
		func(ctx object.Context, self object.Value, name string) ([]string, error) {
			// Resolv::DNS asks the resolver even for an address, as Ruby's
			// does; Resolv.getaddress answers it itself.
			d := dnsOf(self)
			if d.resolver == nil {
				return nil, noResolver("DNS result has no information for " + name)
			}
			c, cancel := withTimeout(d.timeout)
			defer cancel()
			ips, err := d.resolver.LookupIPAddr(c, name)
			if err != nil {
				return netError(err)
			}
			out := make([]string, len(ips))
			for i, ip := range ips {
				out[i] = ip.IP.String()
			}
			return out, nil
		},
		func(ctx object.Context, self object.Value, addr string) ([]string, error) {
			d := dnsOf(self)
			if d.resolver == nil {
				return nil, noResolver("DNS result has no information for " + addr)
			}
			c, cancel := withTimeout(d.timeout)
			defer cancel()
			names, err := d.resolver.LookupAddr(c, addr)
			if err != nil {
				return netError(err)
			}
			for i, n := range names {
				names[i] = strings.TrimSuffix(n, ".")
			}
			return names, nil
		},
		noInfo, noInfo)
	// getresources(name, typeclass) and getresource.
	resourcesOf := func(ctx object.Context, self object.Value, args []object.Value) ([]object.Value, string, error) {
		if len(args) != 2 {
			return nil, "", stdlib.ArgError(ctx, len(args), "2")
		}
		name, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return nil, "", err
		}
		typ, err := resourceType(ctx, args[1])
		if err != nil {
			return nil, "", err
		}
		d := dnsOf(self)
		if d.resolver == nil {
			return nil, name, noResolver("DNS result has no information for " + name)
		}
		c, cancel := withTimeout(d.timeout)
		defer cancel()
		r := d.resolver
		var out []object.Value
		switch typ {
		case "A", "AAAA":
			ips, err := r.LookupIPAddr(c, name)
			if err != nil {
				if _, err := netError(err); err != nil {
					return nil, name, err
				}
				return nil, name, nil
			}
			for _, ip := range ips {
				if ip4 := ip.IP.To4(); (ip4 != nil) == (typ == "A") {
					if typ == "A" {
						out = append(out, instance(resources["A"], object.String(ip4.String())))
					} else {
						out = append(out, instance(resources["AAAA"], object.String(ipv6String(ip.IP))))
					}
				}
			}
		case "MX":
			mxs, err := r.LookupMX(c, name)
			if err != nil {
				if _, err := netError(err); err != nil {
					return nil, name, err
				}
				return nil, name, nil
			}
			for _, mx := range mxs {
				out = append(out, instance(resources["MX"], object.Int(int64(mx.Pref)), object.String(strings.TrimSuffix(mx.Host, "."))))
			}
		case "TXT":
			txts, err := r.LookupTXT(c, name)
			if err != nil {
				if _, err := netError(err); err != nil {
					return nil, name, err
				}
				return nil, name, nil
			}
			for _, t := range txts {
				out = append(out, instance(resources["TXT"], object.String(t)))
			}
		case "CNAME":
			cname, err := r.LookupCNAME(c, name)
			if err != nil {
				if _, err := netError(err); err != nil {
					return nil, name, err
				}
				return nil, name, nil
			}
			if target := strings.TrimSuffix(cname, "."); target != "" && !strings.EqualFold(target, strings.TrimSuffix(name, ".")) {
				out = append(out, instance(resources["CNAME"], object.String(target)))
			}
		case "NS":
			nss, err := r.LookupNS(c, name)
			if err != nil {
				if _, err := netError(err); err != nil {
					return nil, name, err
				}
				return nil, name, nil
			}
			sort.Slice(nss, func(i, j int) bool { return nss[i].Host < nss[j].Host })
			for _, ns := range nss {
				out = append(out, instance(resources["NS"], object.String(strings.TrimSuffix(ns.Host, "."))))
			}
		case "PTR":
			names, err := r.LookupAddr(c, strings.TrimSuffix(name, "."))
			if err != nil {
				if _, err := netError(err); err != nil {
					return nil, name, err
				}
				return nil, name, nil
			}
			for _, n := range names {
				out = append(out, instance(resources["PTR"], object.String(strings.TrimSuffix(n, "."))))
			}
		case "SRV":
			_, srvs, err := r.LookupSRV(c, "", "", name)
			if err != nil {
				if _, err := netError(err); err != nil {
					return nil, name, err
				}
				return nil, name, nil
			}
			for _, s := range srvs {
				out = append(out, instance(resources["SRV"], object.Int(int64(s.Priority)), object.Int(int64(s.Weight)), object.Int(int64(s.Port)), object.String(strings.TrimSuffix(s.Target, "."))))
			}
		default:
			return nil, name, ctx.Raise(object.ArgumentError, "Resolv::DNS::Resource::IN::%s is not answered by the host's resolver", typ)
		}
		return out, name, nil
	}
	c.Define("getresources", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out, _, err := resourcesOf(ctx, self, args)
		if err != nil {
			return object.Value{}, err
		}
		return object.NewList(out...), nil
	})
	c.Define("getresource", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out, name, err := resourcesOf(ctx, self, args)
		if err != nil {
			return object.Value{}, err
		}
		if len(out) == 0 {
			return object.Value{}, noInfo(name)
		}
		return out[0], nil
	})
	c.Define("each_resource", 2, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		out, _, err := resourcesOf(ctx, self, args)
		if err != nil {
			return object.Value{}, err
		}
		for _, r := range out {
			if _, err := ctx.Yield(block, r); err != nil {
				return object.Value{}, err
			}
		}
		return object.Nil, nil
	})
}
