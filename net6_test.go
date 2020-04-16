package iplib

import (
	"bytes"
	"math/big"
	"net"
	"testing"
)

var Network6Tests = []struct {
	inaddrStr  string
	ipaddr     net.IP
	inaddrMask int
	firstaddr  net.IP
	lastaddr   net.IP
	count      string // might overflow uint64
}{
	{
		"2001:db8::/64",
		net.ParseIP("2001:db8::"),
		64,
		net.ParseIP("2001:db8::"),
		net.ParseIP("2001:db8::ffff:ffff:ffff"),
		"18446744073709551616",
	},
	{
		"2001:db8::/72",
		net.ParseIP("2001:db8::ffff"),
		72,
		net.ParseIP("2001:db8::1"),
		net.ParseIP("2001:0db8:0000:0000:00ff:ffff:ffff:ffff"),
		"72057594037927936",
	},
	{
		"::",
		net.ParseIP("::"),
		64,
		net.ParseIP("::"),
		net.ParseIP("::ffff:ffff:ffff"),
		"18446744073709551616",
	},
	{
		"2001::db8::/127",
		net.ParseIP("2001:db8:0:11::"),
		127,
		net.ParseIP("2001:db8:0:11::"),
		net.ParseIP("2001:db8:0:11::1"),
		"2",
	},
}

func TestNet6_Version(t *testing.T) {
	for _, tt := range Network6Tests {
		_, ipnp, _ := ParseCIDR(tt.inaddrStr)
		ipnn := NewNet(tt.ipaddr, tt.inaddrMask)
		if ipnp.Version() != 6 {
			t.Errorf("From ParseCIDR %s got Network.Version == %d, expect 6", tt.inaddrStr, ipnp.Version())
		}
		if ipnn.Version() != 6 {
			t.Errorf("From NewNet %s got Network.Version == %d, expect 6", tt.inaddrStr, ipnn.Version())
		}
	}
}

func TestNet6_Count(t *testing.T) {
	for _, tt := range Network6Tests {
		_, ipn, _ := ParseCIDR(tt.inaddrStr)
		ipn6 := ipn.(Net6)
		count, _ := big.NewInt(0).SetString(tt.count, 10)
		val := count.Cmp(ipn6.Count())
		if val != 0 {
			t.Errorf("On %s got Network.Count == %s, want %s", tt.inaddrStr, ipn6.Count().String(), tt.count)
		}
	}
}

func TestNet6_FirstAddress(t *testing.T) {
	for _, tt := range Network6Tests {
		_, ipn, _ := ParseCIDR(tt.inaddrStr)
		if addr := ipn.FirstAddress(); !tt.firstaddr.Equal(addr) {
			t.Errorf("On %s got Network.FirstAddress == %v, want %v", tt.inaddrStr, addr, tt.firstaddr)
		}
	}
}

func TestNet6_LastAddress(t *testing.T) {
	for _, tt := range Network6Tests {
		_, ipn, _ := ParseCIDR(tt.inaddrStr)
		if addr := ipn.LastAddress(); !tt.lastaddr.Equal(addr) {
			t.Errorf("On %s got Network.LastAddress == %v, want %v", tt.inaddrStr, addr, tt.lastaddr)
		}
	}
}

var enumerate6Tests = []struct {
	inaddr net.IP
	total string
	last  net.IP
}{
	{},
}

var sortNet6Tests = map[int]string{

}

var compareNet6 = []struct {
	network string
	subnet  string
	result  bool
}{
	{ },
}

func TestNet6_ContainsNeWork(t *testing.T) {
	for _, cidr := range compareNet6 {
		_, ipn, _ := ParseCIDR(cidr.network)
		_, sub, _ := ParseCIDR(cidr.subnet)
		result := ipn.ContainsNet(sub)
		if result != cidr.result {
			t.Errorf("For \"%s contains %s\" expected %v got %v", cidr.network, cidr.subnet, cidr.result, result)
		}
	}
}

var hostMaskTests = []struct {
	masklen int
	mask    net.IPMask
}{
	{
		0,
		net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
	},
	{
		1,
		net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x01},
	},
	{
		2,
		net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x03},
	},
	{
		3,
		net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x07},
	},
	{
		4,
		net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0f},
	},
	{
		5,
		net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1f},
	},
	{
		6,
		net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x3f},
	},
	{
		7,
		net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x7f},
	},
	{
		8,
		net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xff},
	},
	{
		16,
		net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xff, 0xff},
	},
	{
		32,
		net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xff, 0xff, 0xff, 0xff},
	},
	{
		64,
		net.IPMask{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	},
	{
		128,
		net.IPMask{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	},
}

func Test_mkHostMask(t *testing.T) {
	for _, tt := range hostMaskTests {
		mask := mkHostMask(tt.masklen)
		v := bytes.Compare(mask, tt.mask)
		if v != 0 {
			t.Errorf("Got wrong mask value for masklen %d", tt.masklen)
		}
	}
}

var controlsTests = []struct {
	inaddr Net6
	addrs map[string]bool
}{
	{
		NewNet6(net.ParseIP("2001:db8:1::"), 56, 64),
		map[string]bool{
			"2001:db8:1:1::": true,
			"2001:db8:2::": false,
			"2001:db8:1:ff:1::": false,
		},
	},
}

func compareNet6ArraysToStringRepresentation(a []Net6, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i, n := range a {
		if n.String() != b[i] {
			return false
		}
	}

	return true
}