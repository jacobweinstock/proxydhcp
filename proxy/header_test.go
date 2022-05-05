package proxy

import (
	"errors"
	"net"
	"net/url"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
	"inet.af/netaddr"
)

func TestSetMessageType(t *testing.T) {
	tests := []struct {
		name      string
		mType     dhcpv4.MessageType
		wantMType dhcpv4.MessageType
		wantErr   error
	}{
		{
			name:      "success discover packet",
			mType:     dhcpv4.MessageTypeDiscover,
			wantMType: dhcpv4.MessageTypeOffer,
			wantErr:   nil,
		},
		{
			name:      "success request packet",
			mType:     dhcpv4.MessageTypeRequest,
			wantMType: dhcpv4.MessageTypeAck,
			wantErr:   nil,
		},
		{
			name:    "failure inform packet",
			mType:   dhcpv4.MessageTypeInform,
			wantErr: ErrIgnorePacket{PacketType: dhcpv4.MessageTypeInform},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reply := replyPacket{
				DHCPv4: &dhcpv4.DHCPv4{},
				log:    logr.Discard(),
			}
			req := &dhcpv4.DHCPv4{}
			req.UpdateOption(dhcpv4.OptMessageType(tt.mType))
			if err := reply.setMessageType(req); !errors.Is(tt.wantErr, err) {
				t.Logf("want: %T, got: %T", tt.wantErr, err)
				t.Fatalf("replyPacket.setMessageType(): error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				if diff := cmp.Diff(reply.MessageType(), tt.wantMType); diff != "" {
					t.Fatalf(diff)
				}
			}
		})
	}
}

func TestSetSname(t *testing.T) {
	tests := []struct {
		name               string
		opt60              []byte
		tftp               net.IP
		http               net.IP
		wantServerHostName string
	}{
		{
			name:               "ServerHostName set to http ip",
			opt60:              []byte("HTTPClient:Arch:xxxxx:UNDI:yyyzzz"),
			http:               net.IPv4(4, 3, 2, 1),
			wantServerHostName: "4.3.2.1",
		},
		{
			name:               "ServerHostName set to tftp ip",
			opt60:              []byte("PXEClient:Arch:xxxxx:UNDI:yyyzzz"),
			tftp:               net.IPv4(1, 2, 3, 4),
			wantServerHostName: "1.2.3.4",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reply := replyPacket{
				DHCPv4: &dhcpv4.DHCPv4{},
				log:    logr.Discard(),
			}
			reply.setSNAME(tt.opt60, tt.tftp, tt.http)
			if diff := cmp.Diff(reply.ServerHostName, tt.wantServerHostName); diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}

func TestSetBootfile(t *testing.T) {
	mac := net.HardwareAddr{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}
	tests := map[string]struct {
		mach             machine
		customUClass     string
		tftp             netaddr.IPPort
		ipxe             *url.URL
		iscript          string
		wantBootFileName string
		wantErr          error
	}{
		"success - full HTTP location": {
			mach:             machine{mac: mac, arch: iana.EFI_X86_64_HTTP, uClass: Tinkerbell},
			ipxe:             &url.URL{Scheme: "http", Host: "192.168.2.3"},
			iscript:          "auto.ipxe",
			wantBootFileName: "http://192.168.2.3/auto.ipxe",
			wantErr:          nil,
		},
		"success - full TFTP location": {
			mach:             machine{mac: mac, arch: iana.EFI_X86_64, uClass: IPXE},
			tftp:             netaddr.IPPortFrom(netaddr.IPv4(1, 2, 3, 4), 69),
			wantBootFileName: "tftp://1.2.3.4:69/ipxe.efi",
			wantErr:          nil,
		},
		"success - mac/filename URI": {
			mach:             machine{mac: mac, arch: iana.EFI_X86_64},
			wantBootFileName: "ipxe.efi",
			wantErr:          nil,
		},
		"success - httpClient full http URL": {
			mach:             machine{mac: mac, arch: iana.EFI_ARM32_HTTP, cType: httpClient},
			ipxe:             &url.URL{Scheme: "http", Host: "127.0.0.1"},
			wantBootFileName: "http://127.0.0.1/snp.efi",
			wantErr:          nil,
		},
		"failure - no architecture found": {
			mach:    machine{mac: mac, arch: iana.UBOOT_ARM32},
			wantErr: ErrArchNotFound{Arch: iana.UBOOT_ARM32},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			reply := replyPacket{
				DHCPv4: &dhcpv4.DHCPv4{},
				log:    logr.Discard(),
			}
			err := reply.setBootfile(tt.mach, tt.customUClass, tt.tftp, tt.ipxe, tt.iscript)
			if err != nil {
				if diff := cmp.Diff(err, tt.wantErr); diff != "" {
					t.Fatalf(diff)
				}
			}
			if diff := cmp.Diff(reply.DHCPv4.BootFileName, tt.wantBootFileName); diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}
