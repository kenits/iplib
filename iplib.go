/*
Package iplib provides enhanced tools for working with IP networks and
addresses. These tools are built upon and extend the generic functionality
found in the Go "net" package.

The main library comes in two parts: a series of utilities for working with
net.IP (sort, increment, decrement, delta, compare, convert to binary or hex-
string, convert between net.IP and integer) and an enhancement of net.IPNet
called iplib.Net that can calculate the first and last IPs of a block as well
as enumerating the block into []net.IP, incrementing and decrementing within
the boundaries of the block and creating sub- or super-nets of it.

For most features iplib exposes a v4 and a v6 variant to handle each network
properly, but in all cases there is a generic function that handles any IP and
routes between them. One caveat to this is those functions that require or
return an integer value representing the address, in these cases the IPv4
variants take an int32 as input while the IPv6 functions require a *big.Int
in order to work with the 128bits of address.

For functions where it is possible to exceed the address-space the rule is
that underflows return the version-appropriate all-zeroes address while
overflows return the all-ones.

There are also two submodules under iplib: the iplib/iid module contains
functions for generating RFC 7217-compliant IPv6 Interface ID addresses, and
iplib/iana imports the IANA IP Special Registries and exposes functions for
comparing IP addresses against those registries to determine if the IP is part
of a special reservation (for example RFC 1918 private networks or the RFC 3849
documentation network).

A special note about IP blocks with one host bit set (/31, /127): RFC3021 (v4)
and RFC6164 (v6) describe a case for using these netblocks to number each end
of a point-to-point link between routers. In v6 this is outside the normal
limit of a network mask and for v4 it would normally produce a block with no
usable addresses. To satisfy the RFCs the following changes are made:

- Count() will report 2 addresses instead of 0

- FirstAddress() and IP() will be equivalent

- LastAddress() and BroadcastAddress() will be equivalent

*/
package iplib

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
)

const (
	// MaxIPv4 is the max size of a uint32, also the IPv4 address space
	MaxIPv4 = 1<<32 - 1
	MaxUint = 1<<64 - 1
)

var (
	ErrAddressAtEndOfRange = errors.New("proposed operation would cause address to exit block")
	ErrAddressOutOfRange   = errors.New("the given IP address is not a part of this netblock")
	ErrBadMaskLength       = errors.New("illegal mask length provided")
	ErrBroadcastAddress    = errors.New("address is the broadcast address of this netblock (and not considered usable)")
	ErrNetworkAddress      = errors.New("address is the network address of this netblock (and not considered usable)")
	ErrNoValidRange        = errors.New("no netblock can be found between the supplied values")
)

// ByIP implements sort.Interface for net.IP addresses
type ByIP []net.IP

// Len implements sort.interface Len(), returning the length of the
// ByIP array
func (bi ByIP) Len() int {
	return len(bi)
}

// Swap implements sort.interface Swap(), swapping two elements in our array
func (bi ByIP) Swap(a, b int) {
	bi[a], bi[b] = bi[b], bi[a]
}

// Less implements sort.interface Less(), given two elements in the array it
// returns true if the LHS should sort before the RHS. For details on the
// implementation, see CompareIPs()
func (bi ByIP) Less(a, b int) bool {
	val := CompareIPs(bi[a], bi[b])
	if val == -1 {
		return true
	}
	return false
}

// BigintToIP6 converts a big.Int to an ip6 address and returns it as a net.IP
func BigintToIP6(z *big.Int) net.IP {
	b := z.Bytes()
	if len(b) > 16 {
		return generateNetLimits(6, 255)
	}
	if v := z.Sign(); v <= 0 {
		return generateNetLimits(6, 0)
	}

	// for cases where the resulting []byte isn't long enough
	if len(b) < 16 {
		for i := 15 - len(b); i >= 0; i-- {
			b = append([]byte{0}, b...)
		}
	}
	return net.IP(b)
}

// CompareIPs is just a thin wrapper around bytes.Compare, but is here for
// completeness as this is a good way to compare two IP objects. Since it uses
// bytes.Compare the return value is identical: 0 if a==b, -1 if a<b, 1 if a>b
func CompareIPs(a, b net.IP) int {
	return bytes.Compare(a.To16(), b.To16())
}

// CompareNets compares two iplib.Net objects by evaluating their network
// address (the first address in a CIDR range) and, if they're equal,
// comparing their netmasks (smallest wins). This means that if a network is
// compared to one of its subnets, the enclosing network sorts first.
func CompareNets(a, b Net) int {
	val := bytes.Compare(a.IP(), b.IP())
	if val != 0 {
		return val
	}

	am, _ := a.Mask().Size()
	bm, _ := b.Mask().Size()

	if am == bm {
		return 0
	}
	if am < bm {
		return -1
	}
	return 1
}

// DecrementIPBy returns a net.IP that is lower than the supplied net.IP by
// the supplied integer value. If you underflow the IP space it will return
// the zero address.
func DecrementIPBy(ip net.IP, count uint32) net.IP {
	if EffectiveVersion(ip) == 4 {
		return DecrementIP4By(ip, count)
	}
	z := big.NewInt(int64(count))
	return DecrementIP6By(ip, z)
}

// DecrementIP4By returns a v4 net.IP that is lower than the supplied net.IP
// by the supplied integer value. If you underflow the IP space it will return
// 0.0.0.0
func DecrementIP4By(ip net.IP, count uint32) net.IP {
	i := IP4ToUint32(ip)
	d := i - count

	// check for underflow
	if d > i {
		return generateNetLimits(4, 0)
	}
	return Uint32ToIP4(d)
}

// DecrementIP6By returns a net.IP that is lower than the supplied net.IP by
// the supplied integer value. If you underflow the IP space it will return
// ::
func DecrementIP6By(ip net.IP, count *big.Int) net.IP {
	z := IPToBigint(ip)
	z.Sub(z, count)
	return BigintToIP6(z)
}

// DeltaIP takes two net.IP's as input and returns the difference between them
// up to the limit of uint32.
func DeltaIP(a, b net.IP) uint32 {
	if EffectiveVersion(a) == 4 && EffectiveVersion(b) == 4 {
		return DeltaIP4(a, b)
	}
	m := big.NewInt(int64(MaxIPv4))
	z := DeltaIP6(a, b)
	if v := z.Cmp(m); v > 0 {
		return MaxIPv4
	}
	return uint32(z.Uint64())
}

// DeltaIP4 takes two net.IP's as input and returns a total of the number of
// addresses between them, up to the limit of uint32.
func DeltaIP4(a, b net.IP) uint32 {
	ai := IP4ToUint32(a)
	bi := IP4ToUint32(b)

	if ai > bi {
		return ai - bi
	}
	return bi - ai
}

// DeltaIP6 takes two net.IP's as input and returns a total of the number of
// addressed between them as a big.Int. It will technically work on v4 as well
// but is considerably slower than DeltaIP4.
func DeltaIP6(a, b net.IP) *big.Int {
	ai := IPToBigint(a)
	bi := IPToBigint(b)
	i := big.NewInt(0)

	if v := ai.Cmp(bi); v >= 0 {
		return i.Sub(ai, bi)
	}
	return i.Sub(bi, ai)
}

// EffectiveVersion returns 4 if the net.IP either contains a v4 address or if
// it contains the v4-encapsulating v6 address range ::ffff. Note that the
// second example below is a v6 address but reports as v4 because it is in the
// 6-to-4 block. This mirrors how Go's `net` package would treat the address
func EffectiveVersion(ip net.IP) int {
	if ip == nil {
		return 0
	}

	if len(ip) == 4 {
		return 4
	}

	if is6to4(ip) {
		return 4
	}

	return 6
}

// ExpandIP6 takes a net.IP containing an IPv6 address and returns a string of
// the address fully expanded
func ExpandIP6(ip net.IP) string {
	var h []byte
	var s string
	h = make([]byte, hex.EncodedLen(len(ip.To16())))
	hex.Encode(h, []byte(ip))
	for i, c := range h {
		if i%4 == 0 {
			s = s + ":"
		}
		s = s + string(c)
	}
	return s[1:]
}

// ForceIP4 takes a net.IP containing an RFC4291 IPv4-mapped IPv6 address and
// returns only the encapsulated v4 address.
func ForceIP4(ip net.IP) net.IP {
	if len(ip) == 16 {
		return ip[12:]
	}
	return ip
}

// HexStringToIP converts a hexadecimal string to an IP address. If the given
// string cannot be converted nil is returned. Input strings may contain '.'
// or ':'
func HexStringToIP(s string) net.IP {
	normalize := func(c rune) rune {
		if strings.IndexRune(":.", c) == -1 {
			return c
		}
		return -1
	}
	s = strings.Map(normalize, s)
	if len(s) != 8 && len(s) != 32 {
		return nil
	}
	h, err := hex.DecodeString(s)
	if err != nil {
		return nil
	}
	return h
}

// IncrementIPBy returns a net.IP that is greater than the supplied net.IP by
// the supplied integer value. If you overflow the IP space it will return
// the all-ones address
func IncrementIPBy(ip net.IP, count uint32) net.IP {
	if Version(ip) == 4 {
		return IncrementIP4By(ip, count)
	}
	z := big.NewInt(int64(count))
	return IncrementIP6By(ip, z)
}

// IncrementIP4By returns a v4 net.IP that is greater than the supplied
// net.IP by the supplied integer value. If you overflow the IP space it
// will return 255.255.255.255
func IncrementIP4By(ip net.IP, count uint32) net.IP {
	i := IP4ToUint32(ip)
	d := i + count

	// check for overflow
	if d < i {
		return generateNetLimits(4, 255)
	}
	return Uint32ToIP4(d)
}

// IncrementIP6By returns a net.IP that is greater than the supplied net.IP by
// the supplied integer value. If you overflow the IP space it will return the
// (meaningless in this context) all-ones address
func IncrementIP6By(ip net.IP, count *big.Int) net.IP {
	z := IPToBigint(ip)
	z.Add(z, count)
	return BigintToIP6(z)
}

// IPToBinaryString returns the given net.IP as a binary string
func IPToBinaryString(ip net.IP) string {
	var sa []string
	if len(ip) > 4 && EffectiveVersion(ip) == 4 {
		ip = ForceIP4(ip)
	}
	for _, b := range ip {
		sa = append(sa, fmt.Sprintf("%08b", b))
	}
	return strings.Join(sa, ".")
}

// IPToHexString returns the given net.IP as a hexadecimal string. This is the
// default stringer format for v6 net.IP
func IPToHexString(ip net.IP) string {
	if EffectiveVersion(ip) == 4 {
		return hex.EncodeToString(ForceIP4(ip))
	}
	return ip.String()
}

// IP4ToUint32 converts a net.IPv4 to a uint32.
func IP4ToUint32(ip net.IP) uint32 {
	if EffectiveVersion(ip) != 4 {
		return 0
	}

	return binary.BigEndian.Uint32(ForceIP4(ip))
}

// IP6ToUint64 converts a net.IPv6 to a uint64, but only the first 64bits of
// address are considered meaningful (any information in the last 64bits will
// be lost). To work with entire IPv6 addresses use IPToBigint()
func IP6ToUint64(ip net.IP) uint64 {
	if EffectiveVersion(ip) != 6 {
		return 0
	}
	ipn := make([]byte, 8)
	copy(ipn, ip[:8])

	return binary.BigEndian.Uint64(ipn)
}

// IPToARPA takes a net.IP as input and returns a string of the version-
// appropriate ARPA DNS name
func IPToARPA(ip net.IP) string {
	if EffectiveVersion(ip) == 4 {
		return IP4ToARPA(ip)
	}
	return IP6ToARPA(ip)
}

// IP4ToARPA takes a net.IP containing an IPv4 address and returns a string of
// the address represented as dotted-decimals in reverse-order and followed by
// the IPv4 ARPA domain "in-addr.arpa"
func IP4ToARPA(ip net.IP) string {
	ip = ForceIP4(ip)
	return fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa", ip[3], ip[2], ip[1], ip[0])
}

// IP6ToARPA takes a net.IP containing an IPv6 address and returns a string of
// the address represented as a sequence of 4-bit nibbles in reverse order and
// followed by the IPv6 ARPA domain "ip6.arpa"
func IP6ToARPA(ip net.IP) string {
	var domain = "ip6.arpa"
	var h []byte
	var s string
	h = make([]byte, hex.EncodedLen(len(ip)))
	hex.Encode(h, []byte(ip))

	for i := len(h) - 1; i >= 0; i-- {
		s = s + string(h[i]) + "."
	}
	return s + domain
}

// IPToBigint converts a net.IP to big.Int.
func IPToBigint(ip net.IP) *big.Int {
	z := new(big.Int)
	z.SetBytes(ip)
	return z
}

// NextIP returns a net.IP incremented by one from the input address. This
// function is roughly as fast for v4 as IncrementIP4By(1) but is consistently
// 4x faster on v6 than IncrementIP6By(1). The bundled tests provide
// benchmarks doing so, as well as iterating over the entire v4 address space.
func NextIP(ip net.IP) net.IP {
	var ipn []byte
	if Version(ip) == 4 {
		ipn = make([]byte, 4)
		copy(ipn, ip)
	} else {
		ipn = make([]byte, 16)
		copy(ipn, ip)
	}

	for i := len(ipn) - 1; i >= 0; i-- {
		ipn[i]++
		if ipn[i] > 0 {
			return ipn
		}
	}
	return ip // if we're already at the end of range, don't wrap
}

// PreviousIP returns a net.IP decremented by one from the input address. This
// function is roughly as fast for v4 as DecrementIP4By(1) but is consistently
// 4x faster on v6 than DecrementIP6By(1). The bundled tests provide
// benchmarks doing so, as well as iterating over the entire v4 address space.
func PreviousIP(ip net.IP) net.IP {
	var ipn []byte
	if Version(ip) == 4 {
		ipn = make([]byte, 4)
		copy(ipn, ip.To4())
	} else {
		ipn = make([]byte, 16)
		copy(ipn, ip)
	}

	for i := len(ipn) - 1; i >= 0; i-- {
		ipn[i]--
		if ipn[i] != 255 {
			return ipn
		}
	}
	return ip // if we're already at beginning of range, don't wrap
}

// Uint32ToIP4 converts a uint32 to an ip4 address and returns it as a net.IP
func Uint32ToIP4(i uint32) net.IP {
	ip := make([]byte, 4)
	binary.BigEndian.PutUint32(ip, i)
	return ip
}

func Uint64ToIP6(i uint64) net.IP {
	ip := make([]byte, 16)
	binary.BigEndian.PutUint64(ip, i)
	return ip
}

// Version returns 4 if the net.IP contains a v4 address. It will return 6 for
// any v6 address, including the v4-encapsulating v6 address range ::ffff.
// Contrast with EffectiveVersion above and note that in the provided example
// ForceIP4() is used because, by default, net.ParseIP() stores IPv4 addresses
// as 6-to-4 encapsulated v6 addresses. One consequence of which is that
// it is impossible to use a 6-to-4 address as a v6 address
func Version(ip net.IP) int {
	if ip == nil {
		return 0
	}

	if len(ip) == 4 {
		return 4
	}

	return 6
}

func generateNetLimits(version int, filler byte) net.IP {
	var b []byte
	if version == 6 {
		version = 16
	}
	b = make([]byte, version)
	for i := range b {
		b[i] = filler
	}
	return b
}

func is6to4(ip net.IP) bool {
	// addresses prefixed 0000:0000:0000:0000:ffff
	if ip[0] == 0x00 && ip[1] == 0x00 && ip[2] == 0x00 && ip[3] == 0x00 &&
		ip[4] == 0x00 && ip[5] == 0x00 && ip[6] == 0x00 && ip[7] == 0x00 &&
		ip[8] == 0x00 && ip[9] == 0x00 && ip[10] == 0xff && ip[11] == 0xff {
		return true
	}
	return false
}
