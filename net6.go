package iplib

import (
	"errors"
	"math/big"
	"net"
)

var (
	ErrUnsupportedIPVer = errors.New("supplied IP version unsupported")
)

// Net6 is an implementation of Net for IPv6 that conforms to the RIPE "Best
// Current Operational Practice for Operators." This has the following
// implications:
//
// * NewNet6() requires a masklen between 0 - 63
//
// * Enumerate(), NextIP() and PreviousIP() return IP addresses on a 64bit
//   boundary. The idea is that the 128bits of IPv6 address are split between
//   64bits for the network and 64bits for "interface identity." This means
//   that a /48 network has 65,536 addresses, not 1.2 septillion
//
// * Count() returns an int, not a *big.Int, since 65,536 fits easily into
//   a standard integer type while 1.2 septillion would not
//
// * Subnet() behaves in accordance with the guidelines: if the netblock is
//   larger than /48, Subnet() will return /48's when given 0 as an argument,
//   if the netblock is a /48 it will return /56's when given a 0. It is an
//   error to use >63
//
// Net6 is not suitable for assigning /127's to WAN links because it doesn't
// understand /127

// Net6 is an implementation of iplib.Net intended for IPv6 netblocks. The
// most important concept exclusive to Net6 is the idea of network- vs. host-
// bits: it was never envisioned that the internet would need the 320
// undecillion addresses available in the new addressing scheme, instead the
// idea was that some of the address would be used for direct IPv4 replacement
// and the rest would find some other use TBD.
type Net6 struct {
	net.IPNet
	version  int
	length   int
	netbytes int
}

// NewNet6 returns a new Net6 object containing ip at the specified masklen.
func NewNet6(ip net.IP, masklen int) (*Net6, error) {
	version := EffectiveVersion(ip)
	if version != 6 {
		return nil, ErrUnsupportedIPVer
	}
	if masklen > 128 {
		return nil, ErrBadMaskLength
	}

	mask := net.CIDRMask(masklen, 128)
	n := net.IPNet{IP: ip.Mask(mask), Mask: mask}

	return &Net6{IPNet: n, version: version, length: net.IPv6len, netbytes: 8}, nil
}

// Contains returns true if ip is contained in the represented netblock
func (n Net6) Contains(ip net.IP) bool {
	return n.IPNet.Contains(ip)
}

// ContainsNet returns true if the given Net is contained within the
// represented block
func (n Net6) ContainsNet(network Net) bool {
	l1, _ := n.Mask().Size()
	l2, _ := network.Mask().Size()
	return l1 <= l2 && n.Contains(network.IP())
}

// Count returns the number ot IP addresses in the represented netblock
func (n Net6) Count() *big.Int {
	ones, all := n.Mask().Size()
	exp := all - ones
	if exp == 1 {
		return big.NewInt(0)
	}
	if exp == 0 {
		return big.NewInt(1)
	}
	var z, e = big.NewInt(2), big.NewInt(int64(exp))
	return z.Exp(z, e, nil)
}

func (n Net6) Enumerate(size, offset uint64) []net.IP {
	count := uint64(MaxUint)
	if n.Count().IsInt64() {
		count = n.Count().Uint64()
	}

	// offset exceeds total, return an empty array
	if offset > count {
		return []net.IP{}
	}

	// size is greater than the number of addresses that can be returned,
	// adjust the size of the slice but keep going
	if size > (count-offset) || size == 0 {
		size = count - offset
	}

	addrList := make([]net.IP, size)

	addrList[0] = IncrementIP6By(n.FirstAddress(), new(big.Int).SetUint64(offset))
	for i := uint64(1); i <= size-1; i++ {
		addrList[i] = NextIP(addrList[i-1])
	}
	return addrList
}

// FirstAddress returns the first usable address for the represented network
func (n Net6) FirstAddress() net.IP {
	if n.version == 6 {
		return n.IP()
	}
	i, j := n.Mask().Size()
	if i+2 > j {
		return n.IP()
	}
	return NextIP(n.IP())
}

// LastAddress returns the last usable address for the represented network.
// For v6 this is the last address in the block; for v4 it is generally the
// next-to-last address, unless the block is a /31 or /32.
func (n Net6) LastAddress() net.IP {
	a := make([]byte, len(n.IP()))

	// apply wildcard to network, byte by byte
	wc := n.Wildcard()
	for pos, b := range []byte(n.IP()) {
		a[pos] = b + wc[pos]
	}
	return a
}

// Mask returns the netmask of the netblock
func (n Net6) Mask() net.IPMask {
	return n.Mask()
}

// IP returns the network address for the represented network, e.g.
// the lowest IP address in the given block
func (n Net6) IP() net.IP {
	return n.IP()
}

// NextIP takes a net.IP as an argument and attempts to increment it by one
// within the boundary of allocated network-bytes. If the resulting address is
// outside of the range of the represented network it will return an empty
// net.IP and an ErrAddressOutOfRange.
func (n Net6) NextIP(ip net.IP) (net.IP, error) {
	xip := n.nextIPWithNetworkBytes(ip)
	if !n.Contains(xip) {
		return net.IP{}, ErrAddressOutOfRange
	}
	return xip, nil
}

// NextNet takes a CIDR mask-size as an argument and attempts to create a new
// Net object just after the current Net, at the requested mask length
func (n Net6) NextNet(masklen int) (*Net6, error) {
	return NewNet6(NextIP(n.LastAddress()), masklen)
}

// PreviousIP takes a net.IP as an argument and attempts to increment it by
// one within the boundary of the allocated network-bytes. If the resulting
// address is outside the range of the represented netblock it will return an
// empty net.IP and an ErrAddressOutOfRange
func (n Net6) PreviousIP(ip net.IP) (net.IP, error) {
	xip := n.previousIPWithNetworkBytes(ip)
	if !n.Contains(xip) {
		return net.IP{}, ErrAddressAtEndOfRange
	}
	return xip, nil
}

// PreviousNet takes a CIDR mask-size as an argument and creates a new Net
// object just before the current one, at the requested mask length. If the
// specified mask is for a larger network than the current one then the new
// network may encompass the current one
func (n Net6) PreviousNet(masklen int) (*Net6, error) {
	return NewNet6(PreviousIP(n.FirstAddress()), masklen)
}

// SetNetworkBytes sets the number of bytes that are meaningful for routing
// in this netblock (the routing prefix and subnet identifier). The most
// common setting for this would be either 16, meaning the entire block, or 8
// if 64bit interface ID's will be generated for the last half of the address
func (n Net6) SetNetworkBytes(i int) error {
	if i > 8 {
		// return an error
	}
	n.netbytes = i
	return nil
}

// String returns the CIDR notation of the enclosed network e.g. 2001:db8::/16
func (n Net6) String() string {
	return n.IPNet.String()
}

// Subnet takes a CIDR mask-size as an argument and carves the current Net
// object into subnets of that size, returning them as a []Net. The mask
// provided must be a larger-integer than the current mask. If set to 0 Subnet
// will carve the network in half
//
// Examples:
// Net{192.168.1.0/24}.Subnet(0)  -> []Net{192.168.1.0/25, 192.168.1.128/25}
// Net{192.168.1.0/24}.Subnet(26) -> []Net{192.168.1.0/26, 192.168.1.64/26, 192.168.1.128/26, 192.168.1.192/26}
func (n Net6) Subnet(masklen int) ([]*Net6, error) {
	ones, all := n.Mask().Size()
	if ones > masklen {
		return nil, ErrBadMaskLength
	}

	if masklen == 0 {
		masklen = ones + 1
	}

	mask := net.CIDRMask(masklen, all)
	netlist := []*Net6{&Net6{net.IPNet{n.IP(), mask}, n.version, n.length, n.netbytes}}

	for CompareIPs(netlist[len(netlist)-1].LastAddress(), n.LastAddress()) == -1 {
		ng := net.IPNet{IP: NextIP(netlist[len(netlist)-1].LastAddress()), Mask: mask}
		netlist = append(netlist, &Net6{ng, n.version, n.length, n.netbytes})
	}
	return netlist, nil
}

// Supernet takes a CIDR mask-size as an argument and returns a Net object
// containing the supernet of the current Net at the requested mask length.
// The mask provided must be a smaller-integer than the current mask. If set
// to 0 Supernet will return the next-largest network
//
// Examples:
// Net{192.168.1.0/24}.Supernet(0)  -> Net{192.168.0.0/23}
// Net{192.168.1.0/24}.Supernet(22) -> Net{Net{192.168.0.0/22}
func (n Net6) Supernet(masklen int) (*Net6, error) {
	ones, all := n.Mask().Size()
	if ones < masklen {
		return nil, ErrBadMaskLength
	}

	if masklen == 0 {
		masklen = ones - 1
	}

	mask := net.CIDRMask(masklen, all)
	ng := net.IPNet{IP: n.IP().Mask(mask), Mask: mask}
	return &Net6{ng, n.version, n.length, n.netbytes}, nil
}

// Version returns the version of IP for the enclosed netblock as an int. 6
// in this case
func (n Net6) Version() int {
	return n.version
}

// Wildcard will return the wildcard mask for a given netmask
func (n Net6) Wildcard() net.IPMask {
	wc := make([]byte, len(n.Mask()))
	for pos, b := range n.Mask() {
		wc[pos] = 0xff - b
	}
	return wc
}

// nextIPWithNetworkBytes returns the next IP address within the allocated
// network bitmask
func (n Net6) nextIPWithNetworkBytes(ip net.IP) net.IP {
	ipn := make([]byte, 16)
	copy(ipn, ip[:n.netbytes])

	for i := n.netbytes - 1; i >= 0; i-- {
		ipn[i]++
		if ipn[i] > 0 {
			return ipn
		}
	}
	return ip
}

// previousIPWithNetworkBytes returns the previous IP address within the
// allocated network bitmask
func (n Net6) previousIPWithNetworkBytes(ip net.IP) net.IP {
	ipn := make([]byte, 16)
	copy(ipn, ip[:n.netbytes])

	for i := n.netbytes - 1; i >= 0; i-- {
		ipn[i]--
		if ipn[i] != 255 {
			return ipn
		}
	}
	return ip
}
