package proxy

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

func TestSetOpt97(t *testing.T) {
	tests := []struct {
		name     string
		guid     []byte
		wantErr  error
		wantGUID []byte
	}{
		{
			name:    "failure - leading byte must be zero",
			guid:    []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11},
			wantErr: ErrOpt97LeadingByteError,
		},
		{
			name:    "failure - wrong size",
			guid:    []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
			wantErr: ErrOpt97WrongSize,
		},
		{
			name:     "success",
			guid:     []byte{0x00, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11},
			wantErr:  nil,
			wantGUID: []byte{0x00, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10, 0x11},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reply := replyPacket{
				DHCPv4: &dhcpv4.DHCPv4{},
				log:    logr.Discard(),
			}
			err := reply.setOpt97(tt.guid)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("setOpt97() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantGUID != nil {
				if diff := cmp.Diff(reply.GetOneOption(dhcpv4.OptionClientMachineIdentifier), tt.wantGUID); diff != "" {
					t.Fatalf(diff)
				}
			}
		})
	}
}

// Constants for ASCII characters without printable symbols.
// https://github.com/scott-ainsworth/go-ascii/blob/master/ascii.go
const (
	NUL = 0x00 // '\0' Null
	SOH = 0x01 //      Start of Header
	STX = 0x02 //      Start of Text
	ETX = 0x03 //      End of Text
	EOT = 0x04 //      End of Transmission
	ENQ = 0x05 //      Enquiry
	ACK = 0x06 //      Acknowledgement
	BEL = 0x07 // '\a' Bell
	BS  = 0x08 // '\b' Backspace
	HT  = 0x09 // '\t' Horizontal Tab
	LF  = 0x0A // '\n' Line Feed
	VT  = 0x0B // '\v' Vertical Tab
	FF  = 0x0C // '\f' Form Feed
	CR  = 0x0D // '\r' Carriage Return
	SO  = 0x0E //      Shift Out
	SI  = 0x0F //      Shift In
	DLE = 0x10 //      Device Idle
	DC1 = 0x11 //      Device Control 1
	DC2 = 0x12 //      Device Control 2
	DC3 = 0x13 //      Device Control 3
	DC4 = 0x14 //      Device Control 4
	NAK = 0x15 //      Negative Acknowledgement
	SYN = 0x16 //      Synchronize
	ETB = 0x17 //      End of Transmission Block
	CAN = 0x18 //      Cancel
	EM  = 0x19 //      End of Medium
	SUB = 0x1A //      Substitute
	ESC = 0x1B // '\e' Escape
	FS  = 0x1C //      Field Separator
	GS  = 0x1D //      Group Separator
	RS  = 0x1E //      Record Separator
	US  = 0x1F //      Unit Separator
	SP  = 0x20 //      Space
	DEL = 0x7F //      Delete
)

func TestSetOpt43(t *testing.T) {
	empty := []byte{}
	rp := []byte(fmt.Sprintf("%c%c%cRaspberry Pi Boot", NUL, NUL, DC1))
	rp2 := []byte(fmt.Sprintf("%c%c%cPXE", LF, EOT, NUL))
	prefix := []byte{0x06, 0x01, 0x08, 0x09, 0x14}

	tests := []struct {
		name      string
		hw        net.HardwareAddr
		wantOpt43 []byte
	}{
		{
			name:      "success - non raspberry pi",
			hw:        net.HardwareAddr{0x00, 0x01, 0x02, 0x03, 0x04, 0x05},
			wantOpt43: []byte{0x06, 0x01, 0x08},
		},
		{
			name:      "success - with raspberry pi opts",
			hw:        net.HardwareAddr{0xB8, 0x27, 0xEB, 0x03, 0x04, 0x05},
			wantOpt43: append(append(append(empty, prefix...), rp...), rp2...),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reply := replyPacket{
				DHCPv4: &dhcpv4.DHCPv4{},
				log:    logr.Discard(),
			}
			reply.setOpt43(tt.hw)
			if diff := cmp.Diff(reply.GetOneOption(dhcpv4.OptionVendorSpecificInformation), tt.wantOpt43); diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}

func TestSetOpt54(t *testing.T) {
	tests := []struct {
		name   string
		opt60  []byte
		tftp   net.IP
		http   net.IP
		wantIP net.IP
	}{
		{
			name:   "success - PXEClient",
			opt60:  []byte("PXEClient:Arch:xxxxx:UNDI:yyyzzz"),
			tftp:   net.IPv4(192, 168, 1, 1),
			http:   net.IPv4(192, 168, 1, 2),
			wantIP: net.IPv4(192, 168, 1, 1),
		},
		{
			name:   "success - HTTPClient",
			opt60:  []byte("HTTPClient:Arch:xxxxx:UNDI:yyyzzz"),
			tftp:   net.IPv4(192, 168, 1, 1),
			http:   net.IPv4(192, 168, 1, 2),
			wantIP: net.IPv4(192, 168, 1, 2),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reply := replyPacket{
				DHCPv4: &dhcpv4.DHCPv4{},
				log:    logr.Discard(),
			}
			opt54 := reply.setOpt54(tt.opt60, tt.tftp, tt.http)
			if diff := cmp.Diff(opt54, tt.wantIP); diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}
