package proxy

import (
	"context"
	"net"

	"github.com/insomniacslk/dhcp/dhcpv4/server4"
)

func (h *Handler) ServeRedirection(ctx context.Context, addr string) (*server4.Server, error) {
	if err := validate(h); err != nil {
		return nil, err
	}
	h.Log = h.Log.WithName("proxy")

	// for broadcast traffic we need to listen on all IPs
	laddr := net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 67,
	}

	// server4.NewServer() will isolate listening to a specific interface.
	server, err := server4.NewServer(getInterfaceByIP(addr), &laddr, h.Handler)
	if err != nil {
		return nil, err
	}
	return server, nil
}

func (h *Handler) ServeBoot(ctx context.Context, addr string) (*server4.Server, error) {
	if err := validate(h); err != nil {
		return nil, err
	}
	h.Log = h.Log.WithName("proxy")

	laddr := net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 4011,
	}

	server, err := server4.NewServer(getInterfaceByIP(addr), &laddr, h.Secondary)
	if err != nil {
		return nil, err
	}
	return server, nil
}

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
