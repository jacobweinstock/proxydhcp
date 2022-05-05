package proxy

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"inet.af/netaddr"
)

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
	}
	var bootfile string
	// If a machine is in an ipxe boot loop, it is likely to be that we arent matching on IPXE or Tinkerbell.
	// if the "iPXE" user class is found it means we arent in our custom version of ipxe, but because of the option 43 we're setting we need to give a full tftp url from which to boot.
	switch { // order matters here.
	case mach.uClass == Tinkerbell, (customUC != "" && mach.uClass == UserClass(customUC)): // this case gets us out of an ipxe boot loop.
		bootfile = fmt.Sprintf("%s/%s", ipxe, iscript) // ipxe.String()
	case mach.cType == httpClient: // Check the client type from option 60.
		bootfile = fmt.Sprintf("%s/%s", ipxe, bin)
	case mach.uClass == IPXE:
		u := &url.URL{
			Scheme: "tftp",
			Host:   tftp.String(),
			Path:   fmt.Sprintf("%v", bin),
		}
		bootfile = u.String()
	default:
		bootfile = bin
	}
	r.BootFileName = bootfile

	return nil
}
