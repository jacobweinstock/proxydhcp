package proxydhcp

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"inet.af/netaddr"
)

func TestListenAndServe(t *testing.T) {
	tests := map[string]struct {
		h    Handler
		addr netaddr.IPPort
	}{
		"success":    {addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 7676), h: &Noop{}},
		"no handler": {addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 7678)},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Server{}
			ctx, done := context.WithTimeout(context.Background(), time.Millisecond*100)
			defer done()
			go func() {
				<-ctx.Done()
				s.Shutdown()
			}()

			err := s.ListenAndServe(ctx, tt.h)

			switch err.(type) {
			case *net.OpError:
			default:
				t.Fatalf("got: %T, wanted: %T", err, &net.OpError{})
			}
		})
	}
}

func TestServerServe(t *testing.T) {
	tests := map[string]struct {
		h    Handler
		addr netaddr.IPPort
		err  error
	}{
		"success":    {addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 7676), h: &Noop{}},
		"no handler": {addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 7678)},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Server{
				Log:  logr.Discard(),
				Addr: tt.addr,
			}
			ctx, done := context.WithTimeout(context.Background(), time.Millisecond*100)
			defer done()
			go func() {
				<-ctx.Done()
				s.Shutdown()
			}()

			err := s.Serve(ctx, nil)
			switch err.(type) {
			case *net.OpError:
			default:
				if !errors.Is(err, ErrNoConn) {
					t.Fatalf("got: %T, wanted: %T or ErrNoConn", err, &net.OpError{})
				}
			}
		})
	}
}

func TestServe(t *testing.T) {
	tests := map[string]struct {
		h    Handler
		addr netaddr.IPPort
		err  error
	}{
		"success":    {addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 7676), h: &Noop{}},
		"no handler": {addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 7678)},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := &Server{
				Log:  logr.Discard(),
				Addr: tt.addr,
			}
			ctx, done := context.WithTimeout(context.Background(), time.Millisecond*100)
			defer done()
			go func() {
				<-ctx.Done()
				s.Shutdown()
			}()

			err := Serve(ctx, nil, tt.h)
			switch err.(type) {
			case *net.OpError:
			default:
				if !errors.Is(err, ErrNoConn) {
					t.Fatalf("got: %T, wanted: %T or ErrNoConn", err, &net.OpError{})
				}
			}
		})
	}
}
