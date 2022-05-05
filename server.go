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

var ErrNoConn = errors.New("no connection specified")

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
		return ErrNoConn
	}
	dhcp, err := server4.NewServer("", nil, s.handler.Handle, server4.WithConn(c))
	if err != nil {
		return fmt.Errorf("failed to create dhcpv4 server: %w", err)
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
		return fmt.Errorf("failed to merge defaults: %w", err)
	}

	addr := &net.UDPAddr{
		IP:   s.Addr.UDPAddr().IP,
		Port: s.Addr.UDPAddr().Port,
	}
	conn, err := server4.NewIPv4UDPConn("en8", addr)
	if err != nil {
		return fmt.Errorf("failed to create udp connection: %w", err)
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
