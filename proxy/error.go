package proxy

import (
	"fmt"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
)

var (
	// ErrOpt97LeadingByteError is used when the option 60 is not a valid PXE request.
	ErrOpt97LeadingByteError = fmt.Errorf("malformed client GUID (option 97), leading byte must be zero")
	// ErrOpt97WrongSize is used when the option 60 is not a valid PXE request.
	ErrOpt97WrongSize = fmt.Errorf("malformed client GUID (option 97), wrong size")
	// ErrOpt60Missing is used when the option 60 is missing from a PXE request.
	ErrOpt60Missing = fmt.Errorf("not a valid PXE request, missing option 60")
	// ErrOpt93Missing is used when the option 93 is missing from a PXE request.
	ErrOpt93Missing = fmt.Errorf("not a valid PXE request, missing option 93")
	// ErrOpt94Missing is used when the option 94 is missing from a PXE request.
	ErrOpt94Missing = fmt.Errorf("not a valid PXE request, missing option 94")
	// ErrUnknownArch is used when the PXE client request is from an unknown architecture.
	ErrUnknownArch = fmt.Errorf("could not determine client architecture from option 93")
	// ErrInvalidHandler is used when validation of the Handler struct fails.
	ErrInvalidHandler = fmt.Errorf("handler validation failed")
)

// ErrIgnorePacket is for when a DHCP packet should be ignored.
type ErrIgnorePacket struct {
	PacketType dhcpv4.MessageType
	Details    string
}

// Error returns the string representation of ErrIgnorePacket.
func (e ErrIgnorePacket) Error() string {
	return fmt.Sprintf("Ignoring packet: message type %s: details %s", e.PacketType, e.Details)
}

// ErrArchNotFound is for when an PXE client request is an architecture that does not have a matching bootfile.
// See var ArchToBootFile for the look ups.
type ErrArchNotFound struct {
	Arch   iana.Arch
	Detail string
}

// Error returns the string representation of ErrArchNotFound.
func (e ErrArchNotFound) Error() string {
	return fmt.Sprintf("unable to find bootfile for arch %v: details %v", e.Arch, e.Detail)
}

// ErrInvalidMsgType is used when the message type is not a valid DHCP message type [DISCOVER, REQUEST].
type ErrInvalidMsgType struct {
	Invalid dhcpv4.MessageType
}

// Error returns the string representation of ErrInvalidMsgType.
func (e ErrInvalidMsgType) Error() string {
	return fmt.Sprintf("must be a DHCP message of type [DISCOVER, REQUEST], %q", e.Invalid)
}

// ErrInvalidOption60 is used when the option 60 is not a valid PXE request [PXEClient, HTTPClient].
type ErrInvalidOption60 struct {
	Opt60 string
}

// Error returns the string representation of ErrInvalidOption60.
func (e ErrInvalidOption60) Error() string {
	return fmt.Sprintf("not a valid PXE request (option 60 does not start with PXEClient or HTTPClient: %q)", e.Opt60)
}
