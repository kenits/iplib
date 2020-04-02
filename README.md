# IPLib 
[![Documentation](https://godoc.org/github.com/c-robinson/iplib?status.svg)](http://godoc.org/github.com/c-robinson/iplib)
[![CircleCI](https://circleci.com/gh/c-robinson/iplib/tree/master.svg?style=svg)](https://circleci.com/gh/c-robinson/iplib/tree/master)
[![Go Report Card](https://goreportcard.com/badge/github.com/c-robinson/iplib)](https://goreportcard.com/report/github.com/c-robinson/iplib)
[![Coverage Status](https://coveralls.io/repos/github/c-robinson/iplib/badge.svg?branch=master)](https://coveralls.io/github/c-robinson/iplib?branch=master)

I really enjoy Python's [ipaddress](https://docs.python.org/3/library/ipaddress.html)
library and Ruby's [ipaddr](https://ruby-doc.org/stdlib-2.5.1/libdoc/ipaddr/rdoc/IPAddr.html),
I think you can write a lot of neat software if some of the little problems
around manipulating IP addresses and netblocks are taken care of for you, so I
set out to write something like them for my language of choice, Go. This is
what I've come up with.

[IPLib](http://godoc.org/github.com/c-robinson/iplib) is a hopefully useful,
aspirationally full-featured library built around and on top of the address
primitives found in the [net](https://golang.org/pkg/net/) package, it seeks
to make them more accessible and easier to manipulate. 

It includes:

##### net.IP tools

Some simple tools for performing common tasks against IP objects:

- compare two addresses
- get the delta between two addresses
- sort
- decrement or increment addresses
- print addresses as binary or hexadecimal strings, or print their addr.ARPA
  DNS name
- print v6 in fully expanded form
- convert between net.IP and integer values
- get the version of a v4 address or force a IPv4-mapped IPv6address to be a 
  v4 address

##### iplib.Net

An enhancement of `net.IPNet`, `iplib.Net` is an interface with two, version-
specific implementations providing features such as:

- retrieve the first and last usable address
- retrieve the wildcard mask
- enumerate all or part of a netblock to `[]net.IP`
- decrement or increment addresses within the boundaries of the netblock
- return the supernet of a netblock
- allocate subnets within the netblock
- return next- or previous-adjacent netblocks

Additional version-specific considerations described in the [Net4](#using-iplibnet4)
and [Net6](#using-iplibnet6) sections below.

## Sub-modules

- [iana](https://github.com/c-robinson/iplib/tree/master/iana) - a module for referencing 
  IP netblocks against the [Internet Assigned Numbers Authority's](https://www.iana.org/)
  Special IP Address Registry
- [iid](https://github.com/c-robinson/iplib/tree/master/iid) - a module for
  generating and validating IPv6 Interface Identifiers, including [RFC4291](https://tools.ietf.org/html/rfc4291)
  modified EUI64 and [RFC7217](https://tools.ietf.org/html/rfc7217)
  Semantically Opaque addresses

## Installing

```sh
go get -u github.com/c-robinson/iplib
```

## Using iplib

There are a series of functions for working with v4 or v6 `net.IP` objects:

```go
package main

import (
	"fmt"
	"net"
	"sort"
	
	"github.com/c-robinson/iplib"
)


func main() {
	ipa := net.ParseIP("192.168.1.1")
	ipb := iplib.IncrementIPBy(ipa, 15)      // ipb is 192.168.1.16
	ipc := iplib.NextIP(ipa)                 // ipc is 192.168.1.2

	fmt.Println(iplib.CompareIPs(ipa, ipb))  // -1
    
	fmt.Println(iplib.DeltaIP(ipa, ipb))     // 15
    
	fmt.Println(iplib.IPToHexString(ipc))    // "c0a80102"

	iplist := []net.IP{ ipb, ipc, ipa }
	sort.Sort(iplib.ByIP(iplist))            // []net.IP{ipa, ipc, ipb}

	fmt.Println(iplib.IP4ToUint32(ipa))      // 3232235777
	fmt.Println(iplib.IPToBinaryString(ipa))  // 11000000.10101000.00000001.00000001
	ipd := iplib.Uint32ToIP4(iplib.IP4ToUint32(ipa)+20) // ipd is 192.168.1.21
	fmt.Println(iplib.IP4ToARPA(ipa))        // 1.1.168.192.in-addr.arpa
}
```

Addresses that require or return a count default to using `uint32`, which is
sufficient for working with the entire IPv4 space. As a rule these functions
are just lowest-common wrappers around IPv4- or IPv6-specific functions. The
IPv6-specific variants use `big.Int` so they can access the entire v6 space:

## The iplib.Net interface

## Using iplib.Net4

## Using iplib.Net6

The most important feature of `Net6` is the use of host- and network-bits.
Because 2^128th addresses is way more than anyone could possibly need, IPv6
netblocks frequently divide them into bit-groups with defined purposes. A
common purpose being to use part of the block for routing and give part to
the host to form an Interface Identifier (IID).
