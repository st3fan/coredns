package satehdns

import (
	"net/netip"
)

func mustParseIPv4(s string) netip.Addr {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		panic("Invalid IPv4: " + s)
	}

	if !addr.Is4() {
		panic("Invalid IPv4: " + s)
	}

	return addr
}
