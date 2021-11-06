package proxy

import (
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/insomniacslk/dhcp/dhcpv4"
)

// ServeBoot handles dhcp request message types.
// must listen on port 4011.
// 1. listen for generic DHCP packets [conn.RecvDHCP()]
// 2. check if the DHCP packet is requesting PXE boot [isPXEPacket(pkt)]
// 3.
func (h *Handler) Secondary(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	reply, err := dhcpv4.New(dhcpv4.WithReply(m),
		dhcpv4.WithGatewayIP(m.GatewayIPAddr),
		dhcpv4.WithOptionCopied(m, dhcpv4.OptionRelayAgentInformation),
	)
	if err != nil {
		return
	}
	if m.OpCode != dhcpv4.OpcodeBootReply { // TODO(jacobweinstock): dont understand this, found it in an example here: https://github.com/insomniacslk/dhcp/blob/c51060810aaab9c8a0bd1b0fcbf72bc0b91e6427/dhcpv4/server4/server_test.go#L31
		return
	}
	log := h.Log.WithName("secondary")
	log = h.Log.WithValues("hwaddr", m.ClientHWAddr)
	switch mt := m.MessageType(); mt {
	case dhcpv4.MessageTypeRequest:
		if err := isRequestPXEPacket(m); err != nil {
			log.Info("Ignoring packet", "error", err.Error())
			return
		}
		reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
	default:
		log.Info("Ignoring packet", "messageType", mt)
		return
	}

	// TODO add link to intel spec for this needing to be set
	// Set option 43
	opt43(reply, m.ClientHWAddr)

	// Set option 97
	if opt := m.GetOneOption(dhcpv4.OptionClientMachineIdentifier); len(opt) > 0 {
		reply.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClientMachineIdentifier, opt))
	}

	// set broadcast header to true
	reply.SetBroadcast()

	mach, err := processMachine(m)
	if err != nil {
		log.Info("unable to parse arch or user class: unusable packet", "error", err.Error(), "mach", mach)
		return
	}
	log.Info("Got valid request to boot", "hwAddr", mach.mac, "arch", mach.arch, "userClass", mach.uClass)

	// Set option 60
	// The PXE spec says the server should identify itself as a PXEClient
	var opt60 string
	if strings.HasPrefix(string(m.GetOneOption(dhcpv4.OptionClassIdentifier)), string(httpClient)) {
		opt60 = string(httpClient)
	} else {
		opt60 = string(pxeClient)
	}
	reply.UpdateOption(dhcpv4.OptClassIdentifier(string(pxeClient)))

	// Set option 54
	var opt54 net.IP
	if strings.HasPrefix(string(m.GetOneOption(dhcpv4.OptionClassIdentifier)), string(httpClient)) {
		opt54 = h.TFTPAddr.UDPAddr().IP
	} else {
		opt54 = h.HTTPAddr.TCPAddr().IP
	}
	reply.UpdateOption(dhcpv4.OptServerIdentifier(opt54))
	// add the siaddr (IP address of next server) dhcp packet header to a given packet pkt.
	// see https://datatracker.ietf.org/doc/html/rfc2131#section-2
	// without this the pxe client will try to broadcast a request message to 4011
	reply.ServerIPAddr = opt54

	// set sname header
	// see https://datatracker.ietf.org/doc/html/rfc2131#section-2
	var sname string
	switch opt60 {
	case string(pxeClient):
		sname = h.TFTPAddr.IP().String()
	case string(httpClient):
		sname = h.HTTPAddr.IP().String()
	}
	reply.ServerHostName = sname

	// set bootfile header
	// If a machine is in an ipxe boot loop, it is likely to be that we arent matching on IPXE or Tinkerbell
	bin, found := ArchToBootFile[mach.arch]
	if !found {
		log.Info("unable to find bootfile for arch", "arch", mach.arch)
		return
	}
	var bootfile string
	if mach.uClass == IPXE || mach.uClass == Tinkerbell || (h.UserClass != "" && mach.uClass == UserClass(h.UserClass)) {
		bootfile = fmt.Sprintf("%s/%s/%s", h.IPXEAddr, mach.mac.String(), h.IPXEScript)
	} else {
		bootfile = filepath.Join(mach.mac.String(), bin)
	}
	reply.BootFileName = bootfile

	// send the DHCP packet
	if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
		log.Error(err, "failed to send ProxyDHCP offer")
		return
	}
	log.Info("Sent ProxyDHCP offer", "summary", reply.Summary())
}
