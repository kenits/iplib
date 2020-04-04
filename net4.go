package iplib

import (
	"math"
	"net"
)

// Net4 is an implementation of Net intended for IPv4 netblocks. It has
// functions to return the broadcast address and wildcard mask not present in
// the IPv6 implementation
type Net4 struct {
	net.IPNet
	version int
	length  int
}

// NewNet4 returns a new Net4 object containing ip at the specified masklen.
func NewNet4(ip net.IP, masklen int) (Net4, error) {
	var maskMax int
	version := EffectiveVersion(ip)
	if version != 4 {
		return Net4{}, ErrUnsupportedIPVer
	} else {
		maskMax = 32
	}
	mask := net.CIDRMask(masklen, maskMax)
	n := net.IPNet{IP: ip.Mask(mask), Mask: mask}

	return Net4{IPNet: n, version: version, length: net.IPv4len}, nil
}

// BroadcastAddress returns the broadcast address for the represented network.
// In the context of IPv6 broadcast is meaningless and the value will be
// equivalent to LastAddress().
func (n Net4) BroadcastAddress() net.IP {
	a, _ := n.finalAddress()
	return a
}

// Contains returns true if ip is contained in the represented netblock
func (n Net4) Contains(ip net.IP) bool {
	return n.IPNet.Contains(ip)
}

// ContainsNet returns true if the given Net is contained within the
// represented block
func (n Net4) ContainsNet(network Net) bool {
	l1, _ := n.Mask().Size()
	l2, _ := network.Mask().Size()
	return l1 <= l2 && n.Contains(network.IP())
}

// Count returns the total number of usable IP addresses in the represented
// network..
func (n Net4) Count() uint32 {
	ones, all := n.Mask().Size()
	exp := all - ones
	if exp == 1 {
		return uint32(0) // special handling for /31
	}
	if exp == 0 {
		return uint32(1) // special handling for /32
	}
	return uint32(math.Pow(2, float64(exp))) - 2
}

// Enumerate generates an array of all usable addresses in Net up to the
// given size starting at the given offset. If size=0 the entire block is
// enumerated.
//
// NOTE: RFC3021 defines a use case for netblocks of /31 for use in point-to-
// point links. For this reason enumerating networks at these lengths will
// return a 2-element array even though it would naturally return none.
//
// For consistency, enumerating a /32 will return the IP in a 1 element array
func (n Net4) Enumerate(size, offset int) []net.IP {
	count := int(n.Count())

	// offset exceeds total, return an empty array
	if offset > count {
		return []net.IP{}
	}

	// size is greater than the number of addresses that can be returned,
	// adjust the size of the slice but keep going
	if size > (count-offset) || size == 0 {
		size = count - offset
	}

	// Handle edge-case mask sizes
	if count == 1 { // Count() returns 1 if host-bits == 0
		return []net.IP{n.IPNet.IP}

	}
	if count == 0 { // Count() returns 0 if host-bits == 1
		addrList := []net.IP{
			n.IP(),
			n.BroadcastAddress(),
		}

		return addrList[offset:]
	}

	netu := IP4ToUint32(n.FirstAddress())
	netu += uint32(offset)

	addrList := make([]net.IP, size)

	addrList[0] = Uint32ToIP4(netu)
	for i := 1; i <= size-1; i++ {
		addrList[i] = NextIP(addrList[i-1])
	}
	return addrList
}

// FirstAddress returns the first usable address for the represented network
func (n Net4) FirstAddress() net.IP {
	i, j := n.Mask().Size()
	if i+2 > j {
		return n.IPNet.IP
	}
	return NextIP(n.IP())
}

// LastAddress returns the last usable address for the represented network
func (n Net4) LastAddress() net.IP {
	a, ones := n.finalAddress()

	// if it's either a single IP or RFC 3021, return the last address
	if ones >= 31 {
		return a
	}

	return PreviousIP(a)
}

// Mask returns the netmask of the netblock
func (n Net4) Mask() net.IPMask {
	return n.IPNet.Mask
}

// IP returns the network address for the represented network, e.g.
// the lowest IP address in the given block
func (n Net4) IP() net.IP {
	return n.IPNet.IP
}

// NetworkAddress returns the network address for the represented network, e.g.
// the lowest IP address in the given block
func (n Net4) NetworkAddress() net.IP {
	return n.IPNet.IP
}

// NextIP takes a net.IP as an argument and attempts to increment it by one.
// If the input is outside of the range of the represented network it will
// return an empty net.IP and an ErrAddressOutOfRange. If the resulting address
// is out of range it will return an empty net.IP and an ErrAddressAtEndOfRange.
// If the result is the broadcast address, the address _will_ be returned, but
// so will an ErrBroadcastAddress, to indicate that the address is technically
// outside the usable scope
func (n Net4) NextIP(ip net.IP) (net.IP, error) {
	if !n.Contains(ip) {
		return net.IP{}, ErrAddressOutOfRange
	}
	xip := NextIP(ip)
	if !n.Contains(xip) {
		return net.IP{}, ErrAddressAtEndOfRange
	}
	// if this is the broadcast address, return it but warn the caller via error
	if n.BroadcastAddress().Equal(xip) && n.version == 4 {
		return xip, ErrBroadcastAddress
	}
	return xip, nil
}

// NextNet takes a CIDR mask-size as an argument and attempts to create a new
// Net object just after the current Net, at the requested mask length
func (n Net4) NextNet(masklen int) (Net4, error) {
	return NewNet4(NextIP(n.BroadcastAddress()), masklen)
}

// PreviousIP takes a net.IP as an argument and attempts to decrement it by
// one. If the input is outside of the range of the represented network it will
// return an empty net.IP and an ErrAddressOutOfRange. If the resulting address
// is out of range it will return an empty net.IP and ErrAddressAtEndOfRange.
// If the result is the network address, the address _will_ be returned, but
// so will an ErrNetworkAddress, to indicate that the address is technically
// outside the usable scope
func (n Net4) PreviousIP(ip net.IP) (net.IP, error) {
	if !n.Contains(ip) {
		return net.IP{}, ErrAddressOutOfRange
	}
	xip := PreviousIP(ip)
	if !n.Contains(xip) {
		return net.IP{}, ErrAddressAtEndOfRange
	}
	// if this is the network address, return it but warn the caller via error
	if n.IP().Equal(xip) && n.version == 4 {
		return xip, ErrNetworkAddress
	}
	return xip, nil
}

// PreviousNet takes a CIDR mask-size as an argument and creates a new Net
// object just before the current one, at the requested mask length. If the
// specified mask is for a larger network than the current one then the new
// network may encompass the current one, e.g.:
//
// iplib.Net{192.168.4.0/22}.Subnet(21) -> 192.168.0.0/21
//
// In the above case 192.168.4.0/22 is part of 192.168.0.0/21
func (n Net4) PreviousNet(masklen int) (Net4, error) {
	return NewNet4(PreviousIP(n.IP()), masklen)
}

// String returns the CIDR notation of the enclosed network e.g. 192.168.0.1/24
func (n Net4) String() string {
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
func (n Net4) Subnet(masklen int) ([]Net4, error) {
	ones, all := n.Mask().Size()
	if ones > masklen {
		return nil, ErrBadMaskLength
	}

	if masklen == 0 {
		masklen = ones + 1
	}

	mask := net.CIDRMask(masklen, all)
	netlist := []Net4{{net.IPNet{n.IP(), mask}, n.version, n.length}}

	for CompareIPs(netlist[len(netlist)-1].BroadcastAddress(), n.BroadcastAddress()) == -1 {
		ng := net.IPNet{IP: NextIP(netlist[len(netlist)-1].BroadcastAddress()), Mask: mask}
		netlist = append(netlist, Net4{ng, n.version, n.length})
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
func (n Net4) Supernet(masklen int) (Net4, error) {
	ones, all := n.Mask().Size()
	if ones < masklen {
		return Net4{}, ErrBadMaskLength
	}

	if masklen == 0 {
		masklen = ones - 1
	}

	mask := net.CIDRMask(masklen, all)
	ng := net.IPNet{IP: n.IP().Mask(mask), Mask: mask}
	return Net4{ng, n.version, n.length}, nil
}

// Version returns the version of IP for the enclosed netblock, Either 4 or 6.
func (n Net4) Version() int {
	return n.version
}

// Wildcard will return the wildcard mask for a given netmask
func (n Net4) Wildcard() net.IPMask {
	wc := make([]byte, len(n.Mask()))
	for pos, b := range n.Mask() {
		wc[pos] = 0xff - b
	}
	return wc
}

// finalAddress returns the last address in the network. It is private
// because both LastAddress() and BroadcastAddress() rely on it, and both use
// it differently. It returns the last address in the block as well as the
// number of masked bits as an int.
func (n Net4) finalAddress() (net.IP, int) {
	a := make([]byte, len(n.IP()))
	ones, _ := n.Mask().Size()

	// apply wildcard to network, byte by byte
	wc := n.Wildcard()
	for pos, b := range []byte(n.IP()) {
		a[pos] = b + wc[pos]
	}
	return a, ones
}
