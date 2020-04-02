package iplib

import (
	"net"
	"strings"
)

type Net interface {
	Contains(ip net.IP) bool
	ContainsNet(network Net) bool
	Mask() net.IPMask
	IP() net.IP
	Version() int
}

// NewNetBetween takes two net.IP's as input and will return the largest
// netblock that can fit between them (exclusive of the IP's themselves).
// If there is an exact fit it will set a boolean to true, otherwise the bool
// will be false. If no fit can be found (probably because a >= b) an
// ErrNoValidRange will be returned.
func NewNetBetween(a, b net.IP) (Net, bool, error) {
	var exact = false
	v := CompareIPs(a, b)
	if v != -1 {
		return Net{}, exact, ErrNoValidRange
	}

	if Version(a) != Version(b) {
		return Net{}, exact, ErrNoValidRange
	}

	maskMax := 128
	if EffectiveVersion(a) == 4 {
		maskMax = 32
	}

	ipa := NextIP(a)
	ipb := PreviousIP(b)
	for i := 1; i <= maskMax; i++ {
		xnet := NewNet(ipa, i)

		va := CompareIPs(xnet.NetworkAddress(), ipa)
		vb := CompareIPs(xnet.BroadcastAddress(), ipb)
		if va >= 0 && vb <= 0 {
			if va == 0 && vb == 0 {
				exact = true
			}
			return xnet, exact, nil
		}
	}
	return Net{}, exact, ErrNoValidRange
}

// ParseCIDR returns a new Net object. It is a passthrough to net.ParseCIDR
// and will return any error it generates to the caller. There is one major
// difference between how net.IPNet manages addresses and how ipnet.Net does,
// and this function exposes it: net.ParseCIDR *always* returns an IPv6
// address; if given a v4 address it returns the RFC4291 IPv4-mapped IPv6
// address internally, but treats it like v4 in practice. In contrast
// iplib.ParseCIDR will re-encode it as a v4
func ParseCIDR(s string) (net.IP, Net, error) {
	ip, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return ip, nil, err
	}
	masklen, _ := ipnet.Mask.Size()

	if strings.Contains(s, ".") {
		//masklen, _ := ipnet.Mask.Size()
		n, err := NewNet4(ForceIP4(ip), masklen)
		return ForceIP4(ip), n, err
	}

	n, err := NewNet6(ip, masklen)
	return ip, n, err
}
