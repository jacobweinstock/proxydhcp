package proxy

import (
	"context"
	"errors"
	"net/url"
	"testing"

	"github.com/go-logr/logr"
	"inet.af/netaddr"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		handler *Handler
		wantErr error
	}{
		{
			name: "valid host",
			handler: &Handler{
				Ctx:        context.TODO(),
				Log:        logr.Discard(),
				IPXEScript: "auto.ipxe",
				UserClass:  "iPXE",
				IPXEAddr: &url.URL{
					Host: "192.168.2.2",
				},
				TFTPAddr: netaddr.IPPortFrom(netaddr.IPv4(192, 168, 2, 2), 69),
				HTTPAddr: netaddr.IPPortFrom(netaddr.IPv4(192, 168, 2, 2), 80),
			},
			wantErr: nil,
		},
		{
			name:    "invalid",
			handler: &Handler{},
			wantErr: ErrInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validate(tt.handler); !errors.Is(err, tt.wantErr) {
				t.Fatalf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
