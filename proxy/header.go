package proxy

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"inet.af/netaddr"
)

// ensurePXEClient determines if the DHCP request came from a PXE enabled client.
func ensurePXEClient(m *dhcpv4.DHCPv4) error {
	switch mt := m.MessageType(); mt {
	case dhcpv4.MessageTypeDiscover:
		if err := validateDiscover(m); err != nil {
			return ErrIgnorePacket{PacketType: mt, Details: err.Error()}
		}
		return nil
	case dhcpv4.MessageTypeRequest:
		if err := validateRequest(m); err != nil {
			return ErrIgnorePacket{PacketType: mt, Details: err.Error()}
		}
		return nil
	default:
		return ErrIgnorePacket{PacketType: mt, Details: "message type is not supported"}
	}
}

// setMessageType sets the message type (dhcp header).
func (r replyPacket) setMessageType(m *dhcpv4.DHCPv4) error {
	switch mt := m.MessageType(); mt {
	case dhcpv4.MessageTypeDiscover:
		r.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
	case dhcpv4.MessageTypeRequest:
		r.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
	default:
		return ErrIgnorePacket{PacketType: mt}
	}
	return nil
}

// setSNAME sets the server hostname (setSNAME) dhcp header.
func (r replyPacket) setSNAME(reqOpt60 []byte, tftp net.IP, http net.IP) {
	var sname string
	if strings.HasPrefix(string(reqOpt60), string(httpClient)) {
		sname = http.String()
	} else {
		sname = tftp.String()
	}
	r.ServerHostName = sname
}

// setBootfile sets the setBootfile (file) dhcp header. see https://datatracker.ietf.org/doc/html/rfc2131#section-2 .
func (r replyPacket) setBootfile(mach machine, customUC string, tftp netaddr.IPPort, ipxe *url.URL, iscript string) error {
	// set bootfile header
	bin, found := ArchToBootFile[mach.arch]
	if !found {
		return ErrArchNotFound{Arch: mach.arch}
		//return fmt.Errorf("unable to find bootfile for arch %s", mach.arch)
	}
	var bootfile string
	// If a machine is in an ipxe boot loop, it is likely to be that we arent matching on IPXE or Tinkerbell
	// if iPXE user class is found it means we arent in our custom version of ipxe, but because of the option 43 we're setting we need to give a full tftp url from which to boot.
	if mach.uClass == Tinkerbell || (customUC != "" && mach.uClass == UserClass(customUC)) {
		bootfile = fmt.Sprintf("%s/%s/%s", ipxe, mach.mac.String(), iscript)
	} else if mach.uClass == IPXE {
		u := &url.URL{
			Scheme: "tftp",
			Host:   tftp.String(),
			Path:   fmt.Sprintf("%v/%v", mach.mac.String(), bin),
		}
		bootfile = u.String()
	} else {
		bootfile = filepath.Join(mach.mac.String(), bin)
	}
	r.BootFileName = bootfile

	return nil
}
