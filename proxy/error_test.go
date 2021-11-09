package proxy

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
)

func TestIgnorePacket(t *testing.T) {
	p := ErrIgnorePacket{PacketType: dhcpv4.MessageTypeAck, Details: "failure"}
	if diff := cmp.Diff(p.Error(), "Ignoring packet: message type ACK: details failure"); diff != "" {
		t.Errorf(diff)
	}
}

func TestArchNotFound(t *testing.T) {
	p := ErrArchNotFound{Arch: iana.ARC_X86, Detail: "failure"}
	if diff := cmp.Diff(p.Error(), "unable to find bootfile for arch Arc x86: details failure"); diff != "" {
		t.Errorf(diff)
	}
}

func TestInvalidMsgType(t *testing.T) {
	p := ErrInvalidMsgType{Valid: dhcpv4.MessageTypeDiscover, Invalid: dhcpv4.MessageTypeAck}
	if diff := cmp.Diff(p.Error(), "must be a DHCP message of type \"DISCOVER\", \"ACK\""); diff != "" {
		t.Errorf(diff)
	}
}

func TestInvalidOption60(t *testing.T) {
	p := ErrInvalidOption60{Opt60: "failure"}
	if diff := cmp.Diff(p.Error(), "not a valid PXE request (option 60 does not start with PXEClient: \"failure\")"); diff != "" {
		t.Errorf(diff)
	}
}
