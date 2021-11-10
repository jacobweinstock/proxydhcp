package proxy

import (
	"net"
	"strings"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
)

// machine describes a device that is requesting a network boot.
type machine struct {
	mac    net.HardwareAddr
	arch   iana.Arch
	uClass UserClass
}

// name comes from section 2.5 of http://www.pix.net/software/pxeboot/archive/pxespec.pdf
func (h *Handler) Redirection(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	log := h.Log.WithValues("hwaddr", m.ClientHWAddr, "listenAddr", conn.LocalAddr())
	reply, err := dhcpv4.New(dhcpv4.WithReply(m),
		dhcpv4.WithGatewayIP(m.GatewayIPAddr),
		dhcpv4.WithTransactionID(m.TransactionID),
		dhcpv4.WithOptionCopied(m, dhcpv4.OptionRelayAgentInformation),
	)
	if err != nil {
		log.Info("Generating a new transaction id failed, not a problem as we're passing one in, but if this message is showing up a lot then something could be up with github.com/insomniacslk/dhcp")
	}
	if m.OpCode != dhcpv4.OpcodeBootRequest { // TODO(jacobweinstock): dont understand this, found it in an example here: https://github.com/insomniacslk/dhcp/blob/c51060810aaab9c8a0bd1b0fcbf72bc0b91e6427/dhcpv4/server4/server_test.go#L31
		log.Info("Ignoring packet", "OpCode", m.OpCode)
		return
	}
	rp := replyPacket{DHCPv4: reply, log: log}

	if err := rp.validatePXE(m); err != nil {
		log.Info("Ignoring packet: not from a PXE enabled client", "error", err)
		return
	}

	if err := rp.setMessageType(m); err != nil {
		log.Info("Ignoring packet", "error", err.Error())
		return
	}

	mach, err := processMachine(m)
	if err != nil {
		log.Info("unable to parse arch or user class: unusable packet", "error", err.Error(), "mach", mach)
		return
	}

	// Set option 43
	rp.setOpt43(m.ClientHWAddr)

	// Set option 97
	if err := rp.setOpt97(m.GetOneOption(dhcpv4.OptionClientMachineIdentifier)); err != nil {
		log.Info("Ignoring packet", "error", err.Error())
		return
	}

	// set broadcast header to true
	reply.SetBroadcast()

	// Set option 60
	// The PXE spec says the server should identify itself as a PXEClient
	reply.UpdateOption(dhcpv4.OptClassIdentifier(string(pxeClient)))

	// Set option 54
	opt54 := rp.setOpt54(m.GetOneOption(dhcpv4.OptionClassIdentifier), h.TFTPAddr.UDPAddr().IP, h.HTTPAddr.TCPAddr().IP)

	// add the siaddr (IP address of next server) dhcp packet header to a given packet pkt.
	// see https://datatracker.ietf.org/doc/html/rfc2131#section-2
	// without this the pxe client will try to broadcast a request message to port 4011
	reply.ServerIPAddr = opt54

	// set sname header
	// see https://datatracker.ietf.org/doc/html/rfc2131#section-2
	rp.setSNAME(m.GetOneOption(dhcpv4.OptionClassIdentifier), h.TFTPAddr.UDPAddr().IP, h.HTTPAddr.TCPAddr().IP)

	// set bootfile header
	if err := rp.setBootfile(mach, h.UserClass, h.TFTPAddr, h.IPXEAddr, h.IPXEScript); err != nil {
		log.Info("Ignoring packet", "error", err.Error())
		return
	}

	// send the DHCP packet
	if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
		log.Error(err, "failed to send ProxyDHCP offer")
		return
	}
	log.Info("Sent ProxyDHCP offer", "arch", mach.arch, "userClass", mach.uClass, "messageType", rp.MessageType())
}

// validatePXE determines if the DHCP packet meets qualifications of a being a PXE enabled client.
// 1. is a DHCP discovery/request message type
// 2. option 93 is set
// 3. option 94 is set
// 4. option 97 is correct length.
// 5. option 60 is set with this format: "PXEClient:Arch:xxxxx:UNDI:yyyzzz" or "HTTPClient:Arch:xxxxx:UNDI:yyyzzz"
// 6. option 55 is set; only warn if not set
// 7. options 128-135 are set; only warn if not set.
func (r replyPacket) validatePXE(pkt *dhcpv4.DHCPv4) error {
	// only response to DISCOVER and REQUEST packets
	if pkt.MessageType() != dhcpv4.MessageTypeDiscover && pkt.MessageType() != dhcpv4.MessageTypeRequest {
		return ErrInvalidMsgType{Invalid: pkt.MessageType()}
	}
	// option 55 must be set
	if !pkt.Options.Has(dhcpv4.OptionParameterRequestList) {
		// just warn for the moment because we don't actually do anything with this option
		r.log.V(1).Info("warning: missing option 55")
	}
	// option 60 must be set
	if !pkt.Options.Has(dhcpv4.OptionClassIdentifier) {
		return ErrOpt60Missing
	}
	// option 60 must start with PXEClient or HTTPClient
	opt60 := pkt.GetOneOption(dhcpv4.OptionClassIdentifier)
	if !strings.HasPrefix(string(opt60), string(pxeClient)) && !strings.HasPrefix(string(opt60), string(httpClient)) {
		return ErrInvalidOption60{Opt60: string(opt60)}
	}
	// option 93 must be set
	if !pkt.Options.Has(dhcpv4.OptionClientSystemArchitectureType) {
		return ErrOpt93Missing
	}

	// option 94 must be set
	if !pkt.Options.Has(dhcpv4.OptionClientNetworkInterfaceIdentifier) {
		return ErrOpt94Missing
	}

	// option 97 must be have correct length or not be set
	guid := pkt.GetOneOption(dhcpv4.OptionClientMachineIdentifier)
	switch len(guid) {
	case 0:
		// A missing GUID is invalid according to the spec, however
		// there are PXE ROMs in the wild that omit the GUID and still
		// expect to boot. The only thing we do with the GUID is
		// mirror it back to the client if it's there, so we might as
		// well accept these buggy ROMs.
	case 17:
		if guid[0] != 0 {
			return ErrOpt97LeadingByteError
		}
	default:
		return ErrOpt97WrongSize
	}
	// options 128-135 must be set but just warn for now as we're not using them
	// these show up as required in https://www.rfc-editor.org/rfc/rfc4578.html#section-2.4
	opts := []dhcpv4.OptionCode{
		dhcpv4.OptionTFTPServerIPAddress,
		dhcpv4.OptionCallServerIPAddress,
		dhcpv4.OptionDiscriminationString,
		dhcpv4.OptionRemoteStatisticsServerIPAddress,
		dhcpv4.Option8021PVLANID,
		dhcpv4.Option8021QL2Priority,
		dhcpv4.OptionDiffservCodePoint,
		dhcpv4.OptionHTTPProxyForPhoneSpecificApplications,
	}
	for _, opt := range opts {
		if v := pkt.GetOneOption(opt); v == nil {
			r.log.V(1).Info("warning: missing option", "opt", opt)
		}
	}

	return nil
}

// processMachine takes a DHCP packet and returns a populated machine.
func processMachine(pkt *dhcpv4.DHCPv4) (machine, error) {
	mach := machine{}
	// get option 93 ; arch
	fwt := pkt.ClientArch()
	if len(fwt) == 0 {
		return mach, ErrUnknownArch
	}
	// TODO(jacobweinstock): handle unknown arch

	// Basic architecture identification, based purely on
	// the PXE architecture option.
	// https://www.rfc-editor.org/errata_search.php?rfc=4578
	mach.arch = fwt[0]

	// set option 77 from received packet
	mach.uClass = UserClass(string(pkt.GetOneOption(dhcpv4.OptionUserClassInformation)))
	mach.mac = pkt.ClientHWAddr

	return mach, nil
}
