package netprobe

import (
	"net"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func setTTL(conn net.PacketConn, v6 bool, ttl int) error {
	if v6 {
		return ipv6.NewPacketConn(conn).SetHopLimit(ttl)
	}
	return ipv4.NewPacketConn(conn).SetTTL(ttl)
}
