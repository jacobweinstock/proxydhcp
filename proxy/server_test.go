package proxy

import (
	"context"
	"errors"
	"net/url"
	"testing"

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

func TestServer(t *testing.T) {
	tests := []struct {
		name    string
		handler *Handler
		addr    netaddr.IPPort
		wantErr error
	}{
		{
			name: "success",
			handler: &Handler{
				Ctx:      context.Background(),
				Log:      logr.Discard(),
				TFTPAddr: netaddr.IPPortFrom(netaddr.IPv4(127, 0, 0, 1), 69),
				HTTPAddr: netaddr.IPPortFrom(netaddr.IPv4(127, 0, 0, 1), 80),
				IPXEAddr: &url.URL{
					Scheme: "http",
					Host:   "127.0.0.1",
				},
				IPXEScript: "auto.ipxe",
				UserClass:  "",
			},
			addr: netaddr.IPPortFrom(netaddr.IPv4(0, 0, 0, 0), 7679),
		},
		{
			name:    "failure invalid handler struct",
			handler: &Handler{},
			addr:    netaddr.IPPortFrom(netaddr.IPv4(127, 0, 0, 1), 7676),
			wantErr: ErrInvalidHandler,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.handler.Server(tt.addr)
			if !errors.Is(err, tt.wantErr) {
				if err != nil {
					if diff := cmp.Diff(err.Error(), tt.wantErr.Error()); diff != "" {
						t.Fatal(diff)
					}
				} else {
					t.Fatalf("got: %T, wanted: %T", err, tt.wantErr)
				}
			}
		})
	}
}
