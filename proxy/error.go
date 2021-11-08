package proxy

import (
	"fmt"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
)

type ErrIgnorePacket struct {
	PacketType dhcpv4.MessageType
	Details    string
}

func (e ErrIgnorePacket) Error() string {
	return fmt.Sprintf("Ignoring packet: message type %s: details %s", e.PacketType, e.Details)
}

type ErrArchNotFound struct {
	Arch    iana.Arch
	Details string
}

func (e ErrArchNotFound) Error() string {
	return fmt.Sprintf("unable to find bootfile for arch %v: details %v", e.Arch, e.Details)
}
