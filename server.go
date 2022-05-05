package proxydhcp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"

	"github.com/go-logr/logr"
	"github.com/imdario/mergo"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"inet.af/netaddr"
)

var ErrNoHandler = fmt.Errorf("no handler specified. please specify a handler")

// ErrServerClosed is returned by the Server's Serve, ServeTLS, ListenAndServe,
// and ListenAndServeTLS methods after a call to Shutdown or Close.
var ErrServerClosed = errors.New("dhcp: Server closed")

type Server struct {
	Log     logr.Logger
	Addr    netaddr.IPPort
	srvMu   sync.Mutex
	srv     *server4.Server
	handler Handler
}

type Handler interface {
	Handle(net.PacketConn, net.Addr, *dhcpv4.DHCPv4)
}

func Serve(ctx context.Context, c net.PacketConn, h Handler) error {
	srv := &Server{handler: h}

	return srv.Serve(ctx, c)
}

func (s *Server) Serve(ctx context.Context, c net.PacketConn) error {
	if s.handler == nil {
		s.handler = &Noop{}
	}
	if c == nil {
		return fmt.Errorf("no connection specified")
	}
	dhcp, err := server4.NewServer("", nil, s.handler.Handle, server4.WithConn(c))
	if err != nil {
		return err
	}
	s.srvMu.Lock()
	s.srv = dhcp
	s.srvMu.Unlock()

	return s.srv.Serve()
}

func (s *Server) ListenAndServe(ctx context.Context, h Handler) error {
	s.handler = h
	if h == nil {
		s.handler = &Noop{}
	}
	defaults := &Server{
		Log:  logr.Discard(),
		Addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 67),
	}
	if err := mergo.Merge(s, defaults, mergo.WithTransformers(s)); err != nil {
		return err
	}

	addr := &net.UDPAddr{
		IP:   s.Addr.UDPAddr().IP,
		Port: s.Addr.UDPAddr().Port,
	}
	conn, err := server4.NewIPv4UDPConn("en8", addr)
	if err != nil {
		return err
	}

	return s.Serve(ctx, conn)
}

func (s *Server) Shutdown() error {
	s.srvMu.Lock()
	defer s.srvMu.Unlock()
	if s.srv == nil {
		return errors.New("server not running")
	}

	return s.srv.Close()
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

func (s *Server) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	switch typ {
	case reflect.TypeOf(logr.Logger{}):
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				isZero := dst.MethodByName("GetSink")
				result := isZero.Call(nil)
				if result[0].IsNil() {
					dst.Set(src)
				}
			}

			return nil
		}
	case reflect.TypeOf(netaddr.IPPort{}):
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				isZero := dst.MethodByName("IsZero")
				result := isZero.Call([]reflect.Value{})
				if result[0].Bool() {
					dst.Set(src)
				}
			}

			return nil
		}
	}

	return nil
}
