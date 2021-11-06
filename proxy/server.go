package proxy

import (
	"context"
	"net"

	"github.com/go-logr/logr"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"golang.org/x/sync/errgroup"
)

func (h *Handler) Serve(ctx context.Context, addr string) error {
	if h.Log.GetSink() == nil {
		h.Log = logr.Discard()
	}
	h.Log = h.Log.WithName("proxy")

	hd := &Handler{
		Ctx:        ctx,
		Log:        h.Log,
		TFTPAddr:   h.TFTPAddr,
		HTTPAddr:   h.HTTPAddr,
		IPXEAddr:   h.IPXEAddr,
		IPXEScript: h.IPXEScript,
		UserClass:  h.UserClass,
	}

	// for broadcast traffic we need to listen on all IPs
	laddr := net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 67,
	}

	laddr2 := net.UDPAddr{
		IP:   net.ParseIP("0.0.0.0"),
		Port: 4011,
	}

	server2, err := server4.NewServer(getInterfaceByIP(addr), &laddr2, hd.Secondary)
	if err != nil {
		return err
	}

	// server4.NewServer() will isolate listening to a specific interface.
	server, err := server4.NewServer(getInterfaceByIP(addr), &laddr, hd.Handler)
	if err != nil {
		return err
	}

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		h.Log.Info("starting proxydhcp", "addr1", addr, "addr2", "0.0.0.0:67")
		return server.Serve()
	})
	g.Go(func() error {
		h.Log.Info("starting proxydhcp", "addr1", addr, "addr2", "0.0.0.0:4011")
		return server2.Serve()
	})

	errCh := make(chan error)
	go func() {
		errCh <- g.Wait()
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		server.Close()
		server2.Close()
		return nil
	}
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
