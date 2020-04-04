package iplib

import "net"

var Network6Tests = []struct {
	inaddrStr  string
	ipaddr     net.IP
	inaddrMask int
	network    net.IP
	netmask    net.IPMask
	wildcard   net.IPMask
	broadcast  net.IP
	firstaddr  net.IP
	lastaddr   net.IP
	version    int
	count      string // might overflow uint64
}{
	{
		"2001:db8::/64",
		net.IP{32, 1, 13, 184, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		64,
		net.IP{},
		net.IPMask{255, 255, 255, 255, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0, 0, 0},
		net.IPMask{},
		net.IP{},
		net.IP{32, 1, 13, 184, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		net.IP{32, 1, 13, 184, 0, 0, 0, 0, 255, 255, 255, 255, 255, 255, 255, 255},
		6,
		"18446744073709551616",
	},
}