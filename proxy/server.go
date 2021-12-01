package proxy

import (
	"context"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"inet.af/netaddr"
)

// Server returns a proxy DHCP server for the Handler.
func Server(_ context.Context, addr netaddr.IPPort, conn *net.UDPAddr, h server4.Handler) (*server4.Server, error) {
	if conn == nil {
		// for broadcast traffic we need to listen on all IPs
		conn = &net.UDPAddr{
			IP:   net.ParseIP("0.0.0.0"),
			Port: addr.UDPAddr().Port,
		}
	}

	// server4.NewServer() will isolate listening to the specific interface.
	return server4.NewServer(getInterfaceByIP(addr.IP().String()), conn, h)
}

// getInterfaceByIP returns the interface with the given IP address or an empty string.
func getInterfaceByIP(ip string) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.String() == ip {
					return iface.Name
				}
			}
		}
	}
	return ""
}
