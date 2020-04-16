package iplib

import (
	"errors"
	"math/big"
	"net"
)

var (
	ErrUnsupportedIPVer = errors.New("supplied IP version unsupported")
)

// Net6 is an implementation of Net that supports IPv6 operations. To
// initialize a Net6 object you must supply a network address and mask as
// with Net4 but you must also supply an integer value between 0 and 128 that
// Net6 will use to determine what part of the address is given over to
// hosts, which this library will refer to as the "hostmask". If set to a non
// zero value the hostmask is applied in the opposite direction from the
// netmask, and Net6 will only operate on the area between the two masks. An
// example may be helpful:
//
// This case treats the IPv6 block just like a giant IPv4 block with a netmask
// of /56
// n := NewNet6(2001:db8::, 56, 0)
//   Address            2001:db8::
//   Netmask            ffff:ffff:ffff:ff00:0000:0000:0000:0000
//   Hostmask           0000:0000:0000:0000:0000:0000:0000:0000
//   First              2001:0db8:0000:0000:0000:0000:0000:0000
//   Last               2001:0db8:0000:00ff:ffff:ffff:ffff:ffff
//   Count              4722366482869645213696
//
// Here we allocate the same block but with a hostmask of /64. Note that the
// hostmask is applied from the rightmost bit; Net6 will only consider the
// octet between the 56th and 64th bits to be meaningful so will only allocate
// 256 hosts
// n:= NewNet6(2001:db8::, 56, 64)
//   Address            2001:db8::
//   Netmask            ffff:ffff:ffff:ff00:0000:0000:0000:0000
//   Hostmask           0000:0000:0000:0000:ffff:ffff:ffff:ffff
//   First              2001:0db8:0000:0000:0000:0000:0000:0000
//   Last               2001:0db8:0000:00ff:0000:0000:0000:0000
//   Count              256
//
// In the first example the second IP address of the netblock is 2001:db8::1,
// in the second example it is 2001:db8:0:0:1::
//
// Hostmasks affects functions NextIP, PreviousIP, Enumerate, FirstAddress
// and LastAddress; it also affects NextNet and PreviousNet which will inherit
// the hostmask from their parent. Subnet and Supernet both require a hostmask
// in their function calls.
type Net6 struct {
	net.IPNet
	version  int
	length   int
	hostmask net.IPMask
}

// NewNet6 returns an initialized Net6 object at the specified masklen with
// the specified hostmask. If masklen or hostbits is greater than 128 it will
// return an empty object. If a v4 address is supplied it will be trated as a
// RFC4291 v6-encapsulated-v4 network (which is the default behavior for
// net.IP)
func NewNet6(ip net.IP, netmasklen, hostmasklen int) Net6 {
	var maskMax = 128
	if Version(ip) != 6 || netmasklen > maskMax || hostmasklen > maskMax {
		return Net6{IPNet: nil, version:  6, length: net.IPv6len, hostmask: net.IPMask{}}
	}
	netmask := net.CIDRMask(netmasklen, maskMax)
	hostmask := mkHostMask(hostmasklen)

	n := net.IPNet{IP: ip.Mask(netmask), Mask: netmask}
	return Net6{IPNet: n, version: 6, length: net.IPv4len, hostmask: hostmask}
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

// Controls returns true if ip is within the scope of the represented block,
// meaning that it is both inside of the netmask and outside of the hostmask.
// In other words this function will return true if ip would be enumerated by
// this Net6 instance
func (n Net6) Controls(ip net.IP) bool {
	if !n.Contains(ip) {
		return false
	}

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

// Hostmask returns the hostmask of the netblock
func (n Net6) Hostmask() net.IPMask {
	return n.hostmask
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
	return n.IPNet.Mask
}

// IP returns the network address for the represented network, e.g.
// the lowest IP address in the given block
func (n Net6) IP() net.IP {
	return n.IPNet.IP
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
func (n Net6) NextNet(masklen int) Net6 {
	return NewNet6(NextIP(n.LastAddress()), masklen, n.hostbits)
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
func (n Net6) PreviousNet(masklen int) Net6 {
	return NewNet6(PreviousIP(n.FirstAddress()), masklen, n.hostbits)
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
func (n Net6) Subnet(masklen int) ([]Net6, error) {
	ones, all := n.Mask().Size()
	if ones > masklen {
		return nil, ErrBadMaskLength
	}

	if masklen == 0 {
		masklen = ones + 1
	}

	mask := net.CIDRMask(masklen, all)
	netlist := []Net6{{net.IPNet{n.IP(), mask}, n.version, n.length, n.netbytes}}

	for CompareIPs(netlist[len(netlist)-1].LastAddress(), n.LastAddress()) == -1 {
		ng := net.IPNet{IP: NextIP(netlist[len(netlist)-1].LastAddress()), Mask: mask}
		netlist = append(netlist, Net6{ng, n.version, n.length, n.hostmask})
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
func (n Net6) Supernet(masklen int) (Net6, error) {
	ones, all := n.Mask().Size()
	if ones < masklen {
		return Net6{}, ErrBadMaskLength
	}

	if masklen == 0 {
		masklen = ones - 1
	}

	mask := net.CIDRMask(masklen, all)
	ng := net.IPNet{IP: n.IP().Mask(mask), Mask: mask}
	return Net6{ng, n.version, n.length, n.hostmask}, nil
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

func mkHostMask(masklen int) net.IPMask	{
	mask := make([]byte, 16)
	for i := 15; i >= 0; i-- {
		if masklen < 8 {
			mask[i] = ^byte(0xff << masklen)
			break
		}
		mask[i] = 0xff
		masklen -= 8
	}
	return mask
}