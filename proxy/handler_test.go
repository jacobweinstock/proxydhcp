package proxy

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"inet.af/netaddr"
)

func TestValidateHandler(t *testing.T) {
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
				UserClass:  "custom",
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
			wantErr: ErrInvalidHandler,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateHandler(tt.handler); !errors.Is(err, tt.wantErr) {
				t.Fatalf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewHandler(t *testing.T) {
	type args struct {
		ctx   context.Context
		tAddr netaddr.IPPort
		hAddr netaddr.IPPort
		iAddr *url.URL
		opts  []Option
	}
	tests := []struct {
		name string
		args args
		want *Handler
	}{
		{
			name: "good", args: args{ctx: context.Background(),
				opts: []Option{
					WithLogger(logr.Discard()),
					WithIPXEScript("auto.ipxe"),
					WithUserClass("test"),
				},
				tAddr: netaddr.IPPortFrom(netaddr.IPFrom4([4]byte{192, 168, 2, 3}), 69),
				hAddr: netaddr.IPPortFrom(netaddr.IPFrom4([4]byte{192, 168, 2, 3}), 80),
				iAddr: &url.URL{Scheme: "http", Host: "192.168.2.4"},
			},
			want: &Handler{
				Ctx:        context.Background(),
				Log:        logr.Discard(),
				TFTPAddr:   netaddr.IPPortFrom(netaddr.IPFrom4([4]byte{192, 168, 2, 3}), 69),
				HTTPAddr:   netaddr.IPPortFrom(netaddr.IPFrom4([4]byte{192, 168, 2, 3}), 80),
				IPXEAddr:   &url.URL{Scheme: "http", Host: "192.168.2.4"},
				IPXEScript: "auto.ipxe",
				UserClass:  "test",
				Allower:    AllowAll{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewHandler(tt.args.ctx, tt.args.tAddr, tt.args.hAddr, tt.args.iAddr, tt.args.opts...)
			if diff := cmp.Diff(got, tt.want, cmpopts.IgnoreUnexported(logr.Logger{}, netaddr.IPPort{})); diff != "" {
				t.Errorf(diff)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	v := reflect.ValueOf(1)
	if r := validateURL(v); r != nil {
		t.Fatal(r)
	}
	v = reflect.ValueOf(url.URL{
		Host: "\x00",
	})
	if r := validateURL(v); r != nil {
		t.Fatal(r)
	}
}

func TestValidateIPPORT(t *testing.T) {
	v := reflect.ValueOf(1)
	if r := validateIPPORT(v); r != nil {
		t.Fatal(r)
	}
}

func TestValidateLogr(t *testing.T) {
	v := reflect.ValueOf(1)
	if r := validateLogr(v); r != nil {
		t.Fatal(r)
	}
}
