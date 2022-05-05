package proxydhcp

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"inet.af/netaddr"
)

func TestGetInterfaceByIP(t *testing.T) {
	tests := []struct {
		name   string
		ip     string
		wantIF []string
	}{
		{
			name:   "success",
			ip:     "127.0.0.1",
			wantIF: []string{"lo0", "lo"},
		},
		{
			name:   "not found",
			ip:     "1.1.1.1",
			wantIF: []string{""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diffs []string
			for _, want := range tt.wantIF {
				diff := cmp.Diff(getInterfaceByIP(tt.ip), want)
				if diff != "" {
					diffs = append(diffs, diff)
				}
			}
			if len(diffs) == len(tt.wantIF) {
				t.Fatalf("%v", diffs)
			}
		})
	}
}

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

			err := s.ListenAndServe(ctx, tt.h)

			switch err.(type) {
			case *net.OpError:
			default:
				t.Fatalf("got: %T, wanted: %T", err, &net.OpError{})
			}
		})
	}
}
