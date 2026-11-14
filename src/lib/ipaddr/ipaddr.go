// Package ipaddr is Ruby's ipaddr library, built into cameo: IPAddr, an
// IPv4 or IPv6 address with its mask, as a value.
//
// It follows ipaddr 1.2.7, the version Ruby 3.4 ships: IPAddr.new from a
// string with a prefix length or a mask, or from an Integer with its
// family; to_s in Ruby's compressed form, to_string in full, inspect,
// to_i, prefix, netmask, include? and ===, ==, <=> and Comparable, mask,
// to_range, succ, the predicates private?, loopback?, link_local?, ipv4?,
// ipv6?, ipv4_mapped? and ipv4_compat?, the operators | & << >> ~,
// reverse, ip6_arpa, ip6_int, ipv4_mapped, ipv4_compat, native, zone_id,
// hton and IPAddr.ntop, and the error classes. mask! and set are
// mutators; mask returns the masked copy. A Socket module with AF_INET,
// AF_INET6 and AF_UNSPEC is defined for the family constants when the
// core has none.
package ipaddr

import (
	"fmt"
	"github.com/cameo-lang/cameo/src/str"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/cameo-lang/cameo/src/object"
	"github.com/cameo-lang/cameo/src/stdlib"
)

const (
	afUnspec = 0
	afInet   = 2
	afInet6  = 10
)

var (
	class         = object.NewClass("IPAddr", object.ObjectClass)
	errorClass    = object.NewClass("Error", object.ArgumentError)
	invalidAddr   = object.NewClass("InvalidAddressError", errorClass)
	invalidPrefix = object.NewClass("InvalidPrefixError", invalidAddr)
	familyError   = object.NewClass("AddressFamilyError", errorClass)

	in4Mask = new(big.Int).SetUint64(0xffffffff)
	in6Mask = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 128), big.NewInt(1))
	one     = big.NewInt(1)
)

// The fields, in order: an instance is identified by them.
const (
	fFamily = iota
	fAddr
	fMask
	fZone
)

var fieldNames = []string{"family", "addr", "mask_addr", "zone_id"}

// ip is an IPAddr's state.
type ip struct {
	family int
	addr   *big.Int
	mask   *big.Int
	zone   string
}

func (a *ip) bits() int {
	if a.family == afInet6 {
		return 128
	}
	return 32
}

func (a *ip) width() *big.Int {
	if a.family == afInet6 {
		return in6Mask
	}
	return in4Mask
}

// value is the IPAddr of a's state.
func (a *ip) value() object.Value {
	n := a.bits() / 8
	fields := make([]object.Value, len(fieldNames))
	fields[fFamily] = object.Int(a.family)
	fields[fAddr] = object.Bytes(a.addr.FillBytes(make([]byte, n)))
	fields[fMask] = object.Bytes(a.mask.FillBytes(make([]byte, n)))
	fields[fZone] = object.NewString(a.zone, str.UTF8)
	return object.InstanceValue(&object.Instance{Cls: class, Fields: fields})
}

func isIP(v object.Value) bool {
	inst, ok := v.AsInstance()
	return ok && inst.Cls == class
}

func ipOf(v object.Value) *ip {
	f := v.Instance().Fields
	family, _ := f[fFamily].AsInt()
	return &ip{
		family: int(family),
		addr:   new(big.Int).SetBytes(f[fAddr].Bytes()),
		mask:   new(big.Int).SetBytes(f[fMask].Bytes()),
		zone:   f[fZone].Str(),
	}
}

func declareFields() {
	decls := make([]stdlib.FieldDecl, len(fieldNames))
	for i, name := range fieldNames {
		decls[i] = stdlib.FieldDecl{Name: name, Type: "bytes"}
		switch i {
		case fFamily:
			decls[i].Type = "int"
		case fZone:
			decls[i].Type = "string"
		}
	}
	stdlib.DeclareFields(class, decls...)
}

// raise makes one of the library's errors.
func raise(cls *object.Class, format string, args ...any) error {
	return &object.Exception{Cls: cls, Message: fmt.Sprintf(format, args...)}
}

// parse reads an address string with an optional /prefix or /mask, as
// IPAddr.new does. family is the one asked for, afUnspec for either.
func parse(s string, family int) (*ip, error) {
	addr, prefix, hasPrefix := strings.Cut(s, "/")
	a, err := parseAddr(addr, family)
	if err != nil {
		return nil, err
	}
	if !hasPrefix {
		return a, nil
	}
	return a.masked(prefix)
}

// parseAddr reads a bare address: dotted IPv4, or IPv6 with an optional
// zone id.
func parseAddr(s string, family int) (*ip, error) {
	if strings.Count(s, ".") == 3 && !strings.Contains(s, ":") {
		if family == afInet6 {
			return nil, raise(familyError, "address family mismatch")
		}
		n, err := parseV4(s)
		if err != nil {
			return nil, err
		}
		return &ip{family: afInet, addr: n, mask: new(big.Int).Set(in4Mask)}, nil
	}
	if family == afInet {
		return nil, raise(familyError, "address family mismatch")
	}
	zone := ""
	if i := strings.IndexByte(s, '%'); i >= 0 {
		s, zone = s[:i], s[i:]
		// A zone id is word characters, as Ruby's %\w+ reads it.
		if !zoneID.MatchString(zone) {
			return nil, raise(invalidAddr, "invalid address: %s%s", s, zone)
		}
	}
	n, err := parseV6(s)
	if err != nil {
		if zone != "" {
			return nil, raise(invalidAddr, "invalid address: %s%s", s, zone)
		}
		return nil, err
	}
	return &ip{family: afInet6, addr: n, mask: new(big.Int).Set(in6Mask), zone: zone}, nil
}

var zoneID = regexp.MustCompile(`^%[0-9A-Za-z_]+$`)

// parseV4 reads a dotted quad: four decimal octets, none zero-filled.
func parseV4(s string) (*big.Int, error) {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return nil, raise(invalidAddr, "invalid address: %s", s)
	}
	var n uint64
	for _, p := range parts {
		if p == "" || len(p) > 3 {
			return nil, raise(invalidAddr, "invalid address: %s", s)
		}
		for i := 0; i < len(p); i++ {
			if p[i] < '0' || p[i] > '9' {
				return nil, raise(invalidAddr, "invalid address: %s", s)
			}
		}
		if len(p) > 1 && p[0] == '0' {
			return nil, raise(invalidAddr, "zero-filled number in IPv4 address is ambiguous: %s", s)
		}
		v, _ := strconv.Atoi(p)
		if v > 255 {
			return nil, raise(invalidAddr, "invalid address: %s", s)
		}
		n = n<<8 | uint64(v)
	}
	return new(big.Int).SetUint64(n), nil
}

// parseV6 reads an IPv6 address: up to eight groups of one to four hex
// digits, one :: for a run of zero groups, and an embedded dotted quad at
// the end.
func parseV6(s string) (*big.Int, error) {
	bad := raise(invalidAddr, "invalid address: %s", s)
	if s == "" {
		return nil, bad
	}
	var head, tail string
	hasGap := false
	if i := strings.Index(s, "::"); i >= 0 {
		hasGap = true
		head, tail = s[:i], s[i+2:]
		if strings.Contains(tail, "::") {
			return nil, bad
		}
	} else {
		head = s
	}
	groups := func(part string) ([]uint64, error) {
		if part == "" {
			return nil, nil
		}
		var out []uint64
		items := strings.Split(part, ":")
		for i, g := range items {
			if i == len(items)-1 && strings.Contains(g, ".") {
				v4, err := parseV4(g)
				if err != nil {
					return nil, bad
				}
				n := v4.Uint64()
				out = append(out, n>>16, n&0xffff)
				continue
			}
			if g == "" || len(g) > 4 {
				return nil, bad
			}
			v, err := strconv.ParseUint(g, 16, 16)
			if err != nil {
				return nil, bad
			}
			out = append(out, v)
		}
		return out, nil
	}
	h, err := groups(head)
	if err != nil {
		return nil, err
	}
	t, err := groups(tail)
	if err != nil {
		return nil, err
	}
	var all []uint64
	switch {
	case hasGap:
		if len(h)+len(t) > 7 {
			return nil, bad
		}
		all = append(all, h...)
		for len(all)+len(t) < 8 {
			all = append(all, 0)
		}
		all = append(all, t...)
	default:
		if len(h) != 8 {
			return nil, bad
		}
		all = h
	}
	n := new(big.Int)
	for _, g := range all {
		n.Lsh(n, 16)
		n.Or(n, new(big.Int).SetUint64(g))
	}
	return n, nil
}

// masked is the address under a prefix length or a mask string: Ruby's
// mask.
func (a *ip) masked(prefix string) (*ip, error) {
	if n, err := strconv.Atoi(prefix); err == nil && !strings.ContainsAny(prefix, " +-") {
		if len(prefix) > 1 && prefix[0] == '0' {
			return nil, raise(invalidPrefix, "leading zeros in prefix")
		}
		return a.maskLen(n)
	}
	// A mask is an address of its own, read as IPAddr.new reads it.
	m, err := parse(prefix, afUnspec)
	if err != nil {
		return nil, err
	}
	if m.family != a.family {
		return nil, raise(invalidPrefix, "address family is not same")
	}
	return a.maskWith(m.addr, prefix)
}

// maskLen applies a prefix length.
func (a *ip) maskLen(n int) (*ip, error) {
	if n < 0 || n > a.bits() {
		return nil, raise(invalidPrefix, "invalid length: %d", n)
	}
	mask := new(big.Int).Lsh(one, uint(a.bits()-n))
	mask.Sub(mask, one)
	mask.Xor(mask, a.width())
	return &ip{family: a.family, addr: new(big.Int).And(a.addr, mask), mask: mask, zone: a.zone}, nil
}

// maskWith applies a mask given as an address, which must be a run of
// ones followed by zeros.
func (a *ip) maskWith(mask *big.Int, shown string) (*ip, error) {
	if _, ok := prefixOf(mask, a.bits()); !ok {
		return nil, raise(invalidPrefix, "invalid mask %s", shown)
	}
	return &ip{family: a.family, addr: new(big.Int).And(a.addr, mask), mask: new(big.Int).Set(mask), zone: a.zone}, nil
}

// prefixOf is the prefix length of a mask, false when the mask is not
// contiguous ones.
func prefixOf(mask *big.Int, bits int) (int, bool) {
	n := 0
	for ; n < bits; n++ {
		if mask.Bit(bits-1-n) == 0 {
			break
		}
	}
	for i := 0; i < bits-n; i++ {
		if mask.Bit(i) != 0 {
			return 0, false
		}
	}
	return n, true
}

// full is to_string: dotted for IPv4, eight groups of four hex digits
// for IPv6, with the zone id.
func (a *ip) full(n *big.Int) string {
	if a.family == afInet {
		b := n.FillBytes(make([]byte, 4))
		return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
	}
	b := n.FillBytes(make([]byte, 16))
	parts := make([]string, 8)
	for i := range parts {
		parts[i] = fmt.Sprintf("%02x%02x", b[2*i], b[2*i+1])
	}
	return strings.Join(parts, ":")
}

// short is to_s: IPv4 dotted; IPv6 compressed as Ruby compresses it, the
// leftmost longest run of two or more zero groups as ::, and an address
// of the form ::X:Y or ::ffff:X:Y with its last two groups as a dotted
// quad.
func (a *ip) short() string {
	if a.family == afInet {
		return a.full(a.addr)
	}
	b := a.addr.FillBytes(make([]byte, 16))
	groups := make([]int, 8)
	for i := range groups {
		groups[i] = int(b[2*i])<<8 | int(b[2*i+1])
	}
	bestAt, bestLen := -1, 1
	for i := 0; i < 8; {
		if groups[i] != 0 {
			i++
			continue
		}
		j := i
		for j < 8 && groups[j] == 0 {
			j++
		}
		if j-i > bestLen {
			bestAt, bestLen = i, j-i
		}
		i = j
	}
	var parts []string
	for i := 0; i < 8; i++ {
		if i == bestAt {
			parts = append(parts, "")
			i += bestLen - 1
			continue
		}
		parts = append(parts, strconv.FormatInt(int64(groups[i]), 16))
	}
	s := strings.Join(parts, ":")
	if bestAt == 0 {
		s = ":" + s
	}
	if bestAt >= 0 && bestAt+bestLen == 8 {
		s += ":"
	}
	// ::ffff:X:Y and ::X:Y end in a dotted quad, as Ruby's
	// \A::(ffff:)?X:Y\z prints them: ::ffff:1 is ::255.255.0.1, the ffff
	// being X when nothing else can be.
	if rest, ok := strings.CutPrefix(s, "::"); ok {
		for _, prefix := range []string{"::ffff:", "::"} {
			tail, has := strings.CutPrefix("::"+rest, prefix)
			if !has {
				continue
			}
			x, y, found := strings.Cut(tail, ":")
			if !found || strings.Contains(y, ":") {
				continue
			}
			xv, errX := strconv.ParseUint(x, 16, 16)
			yv, errY := strconv.ParseUint(y, 16, 16)
			if errX == nil && errY == nil {
				s = fmt.Sprintf("%s%d.%d.%d.%d", prefix, xv>>8, xv&255, yv>>8, yv&255)
				break
			}
		}
	}
	return s + a.zone
}

func (a *ip) inspect() string {
	name := "IPv4"
	if a.family == afInet6 {
		name = "IPv6"
	}
	return "#<IPAddr: " + name + ":" + a.full(a.addr) + a.zone + "/" + a.full(a.mask) + ">"
}

// other coerces the argument of include?, == and the operators: an
// IPAddr, or a String or Integer read in a's family.
func (a *ip) other(ctx object.Context, v object.Value) (*ip, error) {
	if isIP(v) {
		return ipOf(v), nil
	}
	if s, ok := v.AsString(); ok {
		return parse(s, afUnspec)
	}
	if n, ok := v.AsBigInt(); ok {
		return fromInt(n, a.family)
	}
	if n, ok := v.AsInt(); ok {
		return fromInt(big.NewInt(n), a.family)
	}
	return nil, ctx.Raise(object.TypeError, "can't convert %s into IPAddr", stdlib.KindName(v))
}

// fromInt is an address from its integer, in a family.
func fromInt(n *big.Int, family int) (*ip, error) {
	a := &ip{family: family, addr: new(big.Int).Set(n)}
	switch family {
	case afInet:
		a.mask = new(big.Int).Set(in4Mask)
	case afInet6:
		a.mask = new(big.Int).Set(in6Mask)
	default:
		return nil, raise(familyError, "address family must be specified")
	}
	if n.Sign() < 0 || n.Cmp(a.width()) > 0 {
		return nil, raise(invalidAddr, "invalid address: %s", n.String())
	}
	return a, nil
}

// includes is include?: other's range within a's.
func (a *ip) includes(o *ip) bool {
	if a.family != o.family {
		return false
	}
	mine := new(big.Int).And(a.addr, a.mask)
	begin := new(big.Int).And(o.addr, a.mask)
	end := new(big.Int).Or(o.addr, new(big.Int).Xor(o.mask, o.width()))
	return begin.Cmp(mine) == 0 && new(big.Int).And(end, a.mask).Cmp(mine) == 0
}

func (a *ip) ipv4Mapped() bool {
	return a.family == afInet6 && new(big.Int).Rsh(a.addr, 32).Cmp(big.NewInt(0xffff)) == 0
}

func (a *ip) ipv4Compat() bool {
	if a.family != afInet6 || new(big.Int).Rsh(a.addr, 32).Sign() != 0 {
		return false
	}
	low := new(big.Int).And(a.addr, in4Mask)
	return low.Cmp(one) > 0
}

// v4part is the IPv4 address an IPv6 mapped or compat address embeds.
func (a *ip) v4part() *big.Int { return new(big.Int).And(a.addr, in4Mask) }

func (a *ip) inRange(lo, bits int) bool {
	return new(big.Int).Rsh(a.addr, uint(a.bits()-bits)).Cmp(big.NewInt(int64(lo))) == 0
}

func (a *ip) private() bool {
	switch a.family {
	case afInet:
		return a.inRange(10, 8) || a.inRange(172<<4|1, 12) || a.inRange(192<<8|168, 16)
	case afInet6:
		if a.ipv4Mapped() {
			v4 := &ip{family: afInet, addr: a.v4part(), mask: in4Mask}
			return v4.private()
		}
		return a.inRange(0xfc00>>9, 7)
	}
	return false
}

func (a *ip) loopback() bool {
	switch a.family {
	case afInet:
		return a.inRange(127, 8)
	case afInet6:
		if a.ipv4Mapped() {
			return (&ip{family: afInet, addr: a.v4part(), mask: in4Mask}).loopback()
		}
		return a.addr.Cmp(one) == 0
	}
	return false
}

func (a *ip) linkLocal() bool {
	switch a.family {
	case afInet:
		return a.inRange(169<<8|254, 16)
	case afInet6:
		if a.ipv4Mapped() {
			return (&ip{family: afInet, addr: a.v4part(), mask: in4Mask}).linkLocal()
		}
		return a.inRange(0xfe80>>6, 10)
	}
	return false
}

// reverse is the DNS reverse name.
func (a *ip) reverse() string {
	if a.family == afInet {
		b := a.addr.FillBytes(make([]byte, 4))
		return fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa", b[3], b[2], b[1], b[0])
	}
	return a.ip6Arpa()
}

func (a *ip) nibbles() string {
	hex := fmt.Sprintf("%032x", a.addr)
	parts := make([]string, 32)
	for i := 0; i < 32; i++ {
		parts[i] = string(hex[31-i])
	}
	return strings.Join(parts, ".")
}

func (a *ip) ip6Arpa() string { return a.nibbles() + ".ip6.arpa" }
func (a *ip) ip6Int() string  { return a.nibbles() + ".ip6.int" }

func wantIPv6(a *ip) error {
	if a.family != afInet6 {
		return raise(invalidAddr, "not an IPv6 address: %s", a.addr.String())
	}
	return nil
}

func wantIPv4(a *ip) error {
	if a.family != afInet {
		return raise(invalidAddr, "not an IPv4 address: %s", a.addr.String())
	}
	return nil
}

// Install defines IPAddr, its errors, and Socket's family constants when
// the core has none. lib.Install calls it once.
func Install() {
	c := class
	declareFields()
	object.DefineNative("IPAddr", object.ClassValue(c))
	c.Include(object.Comparable)
	c.Consts["VERSION"] = object.NewString("1.2.7", str.ASCII)
	c.Consts["IN4MASK"] = object.BigInt(in4Mask)
	c.Consts["IN6MASK"] = object.BigInt(in6Mask)
	for _, e := range []*object.Class{errorClass, invalidAddr, invalidPrefix, familyError} {
		e.Lexical = c
		c.Consts[e.Name] = object.ClassValue(e)
	}
	// Socket's address families, which family and the constructor's
	// second argument take; the module is shared with any library that
	// defines more of it.
	socket := stdlib.Module("Socket")
	for name, af := range map[string]int64{"AF_UNSPEC": afUnspec, "AF_INET": afInet, "AF_INET6": afInet6} {
		if _, ok := socket.Consts[name]; !ok {
			socket.Consts[name] = object.Int(af)
		}
	}

	c.DefineSingleton("new", -1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		if len(args) > 2 {
			return object.Value{}, stdlib.ArgError(ctx, len(args), "0..2")
		}
		family := afUnspec
		if len(args) == 2 {
			n, err := stdlib.WantInt(ctx, args[1], "")
			if err != nil {
				return object.Value{}, err
			}
			family = int(n)
			if family != afUnspec && family != afInet && family != afInet6 {
				return object.Value{}, raise(familyError, "unsupported address family: %d", family)
			}
		}
		if len(args) == 0 {
			a, _ := parse("::", afInet6)
			return a.value(), nil
		}
		if n, ok := args[0].AsBigInt(); ok {
			a, err := fromInt(n, family)
			if err != nil {
				return object.Value{}, err
			}
			return a.value(), nil
		}
		if n, ok := args[0].AsInt(); ok {
			a, err := fromInt(big.NewInt(n), family)
			if err != nil {
				return object.Value{}, err
			}
			return a.value(), nil
		}
		if isIP(args[0]) {
			return args[0], nil
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		a, err := parse(s, family)
		if err != nil {
			return object.Value{}, err
		}
		return a.value(), nil
	})
	c.DefineSingleton("ntop", 1, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
		b, ok := args[0].AsBytes()
		if !ok {
			strOf, err := stdlib.WantString(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			b = []byte(strOf)
		}
		switch len(b) {
		case 4:
			return object.NewString((&ip{family: afInet, addr: new(big.Int).SetBytes(b)}).full(new(big.Int).SetBytes(b)), str.UTF8), nil
		case 16:
			return object.NewString((&ip{family: afInet6, addr: new(big.Int).SetBytes(b)}).full(new(big.Int).SetBytes(b)), str.UTF8), nil
		}
		return object.Value{}, raise(familyError, "unsupported address family")
	})

	def := func(name string, arity int, f func(ctx object.Context, a *ip, args []object.Value) (object.Value, error)) {
		c.Define(name, arity, func(ctx object.Context, self object.Value, args []object.Value, kw *object.Map, block object.Value) (object.Value, error) {
			return f(ctx, ipOf(self), args)
		})
	}
	strOf := func(f func(a *ip) string) func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		return func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
			return object.NewString(f(a), str.USASCII), nil
		}
	}
	pred := func(f func(a *ip) bool) func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		return func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
			return object.Bool(f(a)), nil
		}
	}
	def("to_s", 0, strOf((*ip).short))
	def("to_string", 0, strOf(func(a *ip) string { return a.full(a.addr) + a.zone }))
	def("inspect", 0, strOf((*ip).inspect))
	def("netmask", 0, strOf(func(a *ip) string { return a.full(a.mask) }))
	def("reverse", 0, strOf((*ip).reverse))
	def("zone_id", 0, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		if err := wantIPv6(a); err != nil {
			return object.Value{}, err
		}
		if a.zone == "" {
			return object.Nil, nil
		}
		return object.NewString(a.zone, str.UTF8), nil
	})
	def("ip6_arpa", 0, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		if err := wantIPv6(a); err != nil {
			return object.Value{}, err
		}
		return object.NewString(a.ip6Arpa(), str.ASCII), nil
	})
	def("ip6_int", 0, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		if err := wantIPv6(a); err != nil {
			return object.Value{}, err
		}
		return object.NewString(a.ip6Int(), str.ASCII), nil
	})
	def("to_i", 0, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		if a.addr.IsInt64() {
			return object.Int(a.addr.Int64()), nil
		}
		return object.BigInt(new(big.Int).Set(a.addr)), nil
	})
	def("prefix", 0, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		n, _ := prefixOf(a.mask, a.bits())
		return object.Int(n), nil
	})
	def("family", 0, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		return object.Int(a.family), nil
	})
	def("ipv4?", 0, pred(func(a *ip) bool { return a.family == afInet }))
	def("ipv6?", 0, pred(func(a *ip) bool { return a.family == afInet6 }))
	def("ipv4_mapped?", 0, pred((*ip).ipv4Mapped))
	def("ipv4_compat?", 0, pred((*ip).ipv4Compat))
	def("private?", 0, pred((*ip).private))
	def("loopback?", 0, pred((*ip).loopback))
	def("link_local?", 0, pred((*ip).linkLocal))
	def("hton", 0, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		return object.Bytes(a.addr.FillBytes(make([]byte, a.bits()/8))), nil
	})
	includes := func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		o, err := a.other(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		return object.Bool(a.includes(o)), nil
	}
	def("include?", 1, includes)
	def("===", 1, includes)
	// comparable says whether == and <=> read a value as an address: an
	// IPAddr, a String or an Integer.
	comparable := func(v object.Value) bool {
		_, isStr := v.AsString()
		return isIP(v) || isStr || v.IsInteger()
	}
	equal := func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		if !comparable(args[0]) {
			return object.False, nil
		}
		o, err := a.other(ctx, args[0])
		if err != nil {
			return object.False, nil
		}
		return object.Bool(a.family == o.family && a.addr.Cmp(o.addr) == 0), nil
	}
	def("==", 1, equal)
	def("eql?", 1, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		if !isIP(args[0]) {
			return object.False, nil
		}
		o := ipOf(args[0])
		return object.Bool(a.family == o.family && a.addr.Cmp(o.addr) == 0 && a.mask.Cmp(o.mask) == 0 && a.zone == o.zone), nil
	})
	def("<=>", 1, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		if !comparable(args[0]) {
			return object.Nil, nil
		}
		o, err := a.other(ctx, args[0])
		if err != nil || a.family != o.family {
			return object.Nil, nil
		}
		return object.Int(a.addr.Cmp(o.addr)), nil
	})
	def("mask", 1, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		if n, ok := args[0].AsInt(); ok {
			m, err := a.maskLen(int(n))
			if err != nil {
				return object.Value{}, err
			}
			return m.value(), nil
		}
		s, err := stdlib.WantString(ctx, args[0])
		if err != nil {
			return object.Value{}, err
		}
		m, err := a.masked(s)
		if err != nil {
			return object.Value{}, err
		}
		return m.value(), nil
	})
	def("to_range", 0, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		lo := &ip{family: a.family, addr: new(big.Int).And(a.addr, a.mask), mask: a.width(), zone: a.zone}
		hi := &ip{family: a.family, addr: new(big.Int).Or(lo.addr, new(big.Int).Xor(a.mask, a.width())), mask: a.width(), zone: a.zone}
		return object.RangeValue(&object.Range{Lo: lo.value(), Hi: hi.value()}), nil
	})
	// set is Ruby's set: the address with its integer changed, refused
	// when the integer is outside the family, as past 255.255.255.255.
	set := func(a *ip, n *big.Int) (object.Value, error) {
		if n.Sign() < 0 || n.Cmp(a.width()) > 0 {
			return object.Value{}, raise(invalidAddr, "invalid address: %s", n.String())
		}
		return (&ip{family: a.family, addr: n, mask: a.mask, zone: a.zone}).value(), nil
	}
	def("succ", 0, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		return set(a, new(big.Int).Add(a.addr, one))
	})
	binary := func(op func(x, y *big.Int) *big.Int) func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		return func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
			o, err := a.other(ctx, args[0])
			if err != nil {
				return object.Value{}, err
			}
			return set(a, op(a.addr, o.addr))
		}
	}
	def("|", 1, binary(func(x, y *big.Int) *big.Int { return new(big.Int).Or(x, y) }))
	def("&", 1, binary(func(x, y *big.Int) *big.Int { return new(big.Int).And(x, y) }))
	// shift is << and >>: a negative count shifts the other way, as Ruby's
	// Integer shifts. << masks its result to the family, as Ruby's does;
	// >> leaves it to set, so a left shift by a negative count that leaves
	// the family is refused, as Ruby's is.
	shift := func(left bool) func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		return func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
			n, err := stdlib.WantInt(ctx, args[0], "")
			if err != nil {
				return object.Value{}, err
			}
			by := uint(min(max(n, -n), 1<<20))
			r := new(big.Int)
			if left == (n >= 0) {
				r.Lsh(a.addr, by)
			} else {
				r.Rsh(a.addr, by)
			}
			if left {
				r.And(r, a.width())
			}
			return set(a, r)
		}
	}
	def("<<", 1, shift(true))
	def(">>", 1, shift(false))
	def("~", 0, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		return (&ip{family: a.family, addr: new(big.Int).Xor(a.addr, a.width()), mask: a.mask, zone: a.zone}).value(), nil
	})
	def("ipv4_mapped", 0, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		if err := wantIPv4(a); err != nil {
			return object.Value{}, err
		}
		n := new(big.Int).Or(a.addr, new(big.Int).Lsh(big.NewInt(0xffff), 32))
		mask := new(big.Int).Or(a.mask, new(big.Int).Lsh(big.NewInt(0xffff), 32))
		mask.Or(mask, new(big.Int).Lsh(new(big.Int).Sub(new(big.Int).Lsh(one, 80), one), 48))
		return (&ip{family: afInet6, addr: n, mask: mask}).value(), nil
	})
	def("ipv4_compat", 0, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		if err := wantIPv4(a); err != nil {
			return object.Value{}, err
		}
		// Ruby's ipv4_compat changes the family alone: the IPv4 mask stays.
		return (&ip{family: afInet6, addr: new(big.Int).Set(a.addr), mask: new(big.Int).Set(a.mask)}).value(), nil
	})
	def("native", 0, func(ctx object.Context, a *ip, args []object.Value) (object.Value, error) {
		if !a.ipv4Mapped() && !a.ipv4Compat() {
			return a.value(), nil
		}
		return (&ip{family: afInet, addr: a.v4part(), mask: new(big.Int).And(a.mask, in4Mask)}).value(), nil
	})
}
