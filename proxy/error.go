package proxy

import (
	"fmt"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
)

var (
	ErrOpt97LeadingByteError = fmt.Errorf("malformed client GUID (option 97), leading byte must be zero")
	ErrOpt97WrongSize        = fmt.Errorf("malformed client GUID (option 97), wrong size")
	ErrOpt60Missing          = fmt.Errorf("not a valid PXE request, missing option 60")
	ErrOpt93Missing          = fmt.Errorf("not a valid PXE request, missing option 93")
	ErrOpt94Missing          = fmt.Errorf("not a valid PXE request, missing option 94")
	ErrUnknownArch           = fmt.Errorf("could not determine client architecture from option 93")
)

type ErrIgnorePacket struct {
	PacketType dhcpv4.MessageType
	Details    string
}

func (e ErrIgnorePacket) Error() string {
	return fmt.Sprintf("Ignoring packet: message type %s: details %s", e.PacketType, e.Details)
}

type ErrArchNotFound struct {
	Arch   iana.Arch
	Detail string
}

func (e ErrArchNotFound) Error() string {
	return fmt.Sprintf("unable to find bootfile for arch %v: details %v", e.Arch, e.Detail)
}

type ErrInvalidMsgType struct {
	Valid   dhcpv4.MessageType
	Invalid dhcpv4.MessageType
}

func (e ErrInvalidMsgType) Error() string {
	return fmt.Sprintf("must be a DHCP message of type %q, %q", e.Valid, e.Invalid)
}

type ErrInvalidOption60 struct {
	Opt60 string
}

func (e ErrInvalidOption60) Error() string {
	return fmt.Sprintf("not a valid PXE request (option 60 does not start with PXEClient: %q)", e.Opt60)
}
