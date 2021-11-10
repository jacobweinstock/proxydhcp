package proxy

import (
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"inet.af/netaddr"
)

// Server returns a proxy DHCP server for the Handler.
func (h *Handler) Server(addr netaddr.IPPort) (*server4.Server, error) {
	if err := validateHandler(h); err != nil {
		return nil, err
	}
	h.Log = h.Log.WithName("proxy")

	// for broadcast traffic we need to listen on all IPs
	laddr := net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: addr.UDPAddr().Port,
	}

	// server4.NewServer() will isolate listening to a specific interface.
	server, err := server4.NewServer(getInterfaceByIP(addr.IP().String()), &laddr, h.Redirection)
	if err != nil {
		return nil, err
	}
	return server, nil
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
