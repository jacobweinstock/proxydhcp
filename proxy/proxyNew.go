package proxy

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/go-logr/logr"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/pkg/errors"
	"inet.af/netaddr"
)

type Handler struct {
	Ctx        context.Context
	Log        logr.Logger
	TFTPAddr   string
	HTTPAddr   string
	IPXEAddr   string
	IPXEScript string
	UserClass  string
}

func (h *Handler) Handler(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	reply, err := dhcpv4.New(dhcpv4.WithReply(m),
		dhcpv4.WithGatewayIP(m.GatewayIPAddr),
		dhcpv4.WithOptionCopied(m, dhcpv4.OptionRelayAgentInformation),
	)
	if err != nil {
		return
	}
	if m.OpCode != dhcpv4.OpcodeBootRequest { // TODO(jacobweinstock): dont understand this, found it in an example here: https://github.com/insomniacslk/dhcp/blob/c51060810aaab9c8a0bd1b0fcbf72bc0b91e6427/dhcpv4/server4/server_test.go#L31
		return
	}
	log := h.Log.WithValues("hwaddr", m.ClientHWAddr)
	switch mt := m.MessageType(); mt {
	case dhcpv4.MessageTypeDiscover:
		if err = _isDiscoverPXEPacket(m); err != nil {
			log.Info("Ignoring packet", "error", err.Error())
			return
		}

		reply.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
	case dhcpv4.MessageTypeRequest:
		if err := _isRequestPXEPacket(m); err != nil {
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
	_opt43(reply, m.ClientHWAddr)

	// Set option 97
	if opt := m.GetOneOption(dhcpv4.OptionClientMachineIdentifier); len(opt) > 0 {
		reply.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClientMachineIdentifier, opt))
	}

	// withHeaderCiaddr adds the siaddr (IP address of next server) dhcp packet header to a given packet pkt.
	// see https://datatracker.ietf.org/doc/html/rfc2131#section-2
	reply.ServerIPAddr = net.IP{0, 0, 0, 0} // TODO(jacobweinstock): does this need to be null?

	// set broadcast header to true
	reply.SetBroadcast()

	mach, err := _processMachine(m)
	if err != nil {
		log.Info("unable to parse arch or user class: unusable packet", "error", err.Error(), "mach", mach)
		return
	}
	log.Info("Got valid request to boot", "hwAddr", mach.mac, "arch", mach.arch, "userClass", mach.uClass)

	// Set option 60
	// The PXE spec says the server should identify itself as a PXEClient or HTTPClient
	var serverID string
	bootFileName, found := Defaults[mach.arch]
	if found {
		reply.UpdateOption(dhcpv4.OptClassIdentifier(string(pxeClient)))
		// needed for option 54
		u, err := url.Parse(h.TFTPAddr)
		if err != nil {
			log.Info("unable to parse TFTP server address", "error", err.Error())
		}
		serverID = u.Host
	} else {
		reply.UpdateOption(dhcpv4.OptClassIdentifier(string(httpClient)))
		bootFileName = fmt.Sprintf(DefaultsHTTP[mach.arch], h.HTTPAddr)
		// needed for option 54
		serverID = h.HTTPAddr
	}

	// Set option 54
	opt54, err := netaddr.ParseIP(serverID)
	if err != nil {
		log.Info("unable to parse serverID", "error", err.Error(), "serverID", serverID)
	}
	reply.UpdateOption(dhcpv4.OptServerIdentifier(opt54.IPAddr().IP))

	// set sname header
	// see https://datatracker.ietf.org/doc/html/rfc2131#section-2
	reply.ServerHostName = opt54.String()

	// set bootfile header
	// If a machine is in an ipxe boot loop, it is likely to be that we arent matching on IPXE or Tinkerbell
	var bootfile string
	if mach.uClass == IPXE || mach.uClass == Tinkerbell || (h.UserClass != "" && mach.uClass == UserClass(h.UserClass)) {
		bootfile = fmt.Sprintf("%s/%s/%s", h.IPXEAddr, mach.mac.String(), h.IPXEScript)
	} else {
		u, err := url.Parse(h.TFTPAddr)
		if err != nil {
			log.Info("unable to parse TFTP server address", "error", err.Error())
		}
		bootfile = filepath.Join(u.Path, mach.mac.String(), bootFileName)
	}
	reply.BootFileName = bootfile

	// send the DHCP packet
	if _, err := conn.WriteTo(reply.ToBytes(), peer); err != nil {
		log.Error(err, "failed to send ProxyDHCP offer")
		return
	}
	log.Info("Sent ProxyDHCP offer", "summary", reply.Summary())
}

// isDiscoverPXEPacket determines if the DHCP packet meets qualifications of a being a PXE enabled client.
// 1. is a DHCP discovery message type
// 2. option 93 is set
// 3. option 94 is set
// 4. option 97 is correct length.
// 5. option 60 is set with this format: "PXEClient:Arch:xxxxx:UNDI:yyyzzz" or "HTTPClient:Arch:xxxxx:UNDI:yyyzzz"
// 6. option 55 is set; only warn if not set
// 7. options 128-135 are set; only warn if not set.
func _isDiscoverPXEPacket(pkt *dhcpv4.DHCPv4) error {
	// should only be a dhcp discover because a request packet has different requirements
	if pkt.MessageType() != dhcpv4.MessageTypeDiscover {
		return fmt.Errorf("DHCP message type is %s, must be %s", pkt.MessageType(), dhcpv4.MessageTypeDiscover)
	}
	// option 55 must be set
	if !pkt.Options.Has(dhcpv4.OptionParameterRequestList) {
		// just warn for the moment because we don't actually do anything with this option
		fmt.Println("warning: missing option 55")
	}
	// option 60 must be set
	if !pkt.Options.Has(dhcpv4.OptionClassIdentifier) {
		return errors.New("not a PXE boot request (missing option 60)")
	}
	// option 60 must start with PXEClient
	opt60 := pkt.GetOneOption(dhcpv4.OptionClassIdentifier)
	if !strings.HasPrefix(string(opt60), "PXEClient") && !strings.HasPrefix(string(opt60), "HTTPClient") {
		return fmt.Errorf("not a PXE boot request (option 60 does not start with PXEClient: %v)", string(pkt.Options[60]))
	}
	// option 93 must be set
	if !pkt.Options.Has(dhcpv4.OptionClientSystemArchitectureType) {
		return errors.New("not a PXE boot request (missing option 93)")
	}
	// option 94 must be set
	if !pkt.Options.Has(dhcpv4.OptionClientNetworkInterfaceIdentifier) {
		return errors.New("not a PXE boot request (missing option 94)")
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
			return errors.New("malformed client GUID (option 97), leading byte must be zero")
		}
	default:
		return errors.New("malformed client GUID (option 97), wrong size")
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
			fmt.Printf("warning: missing option %d\n", opt)
		}
	}

	return nil
}

// isRequestPXEPacket determines if the DHCP packet meets qualifications of a being a PXE enabled client.
// 1. is a DHCP Request message type
// 2. option 93 is set
// 3. option 94 is set
// 4. option 97 is correct length.
// 5. option 60 is set with this format: "PXEClient:Arch:xxxxx:UNDI:yyyzzz" or "HTTPClient:Arch:xxxxx:UNDI:yyyzzz".
func _isRequestPXEPacket(pkt *dhcpv4.DHCPv4) error {
	// should only be a dhcp request messsage type because a discover message type has different requirements
	if pkt.MessageType() != dhcpv4.MessageTypeRequest {
		return fmt.Errorf("DHCP message type is %s, must be %s", pkt.MessageType(), dhcpv4.MessageTypeRequest)
	}
	// option 60 must be set
	if !pkt.Options.Has(dhcpv4.OptionClassIdentifier) {
		return errors.New("not a PXE boot request (missing option 60)")
	}
	// option 60 must start with PXEClient
	opt60 := pkt.GetOneOption(dhcpv4.OptionClassIdentifier)
	if !strings.HasPrefix(string(opt60), "PXEClient") && !strings.HasPrefix(string(opt60), "HTTPClient") {
		return fmt.Errorf("not a PXE boot request (option 60 does not start with PXEClient: %v)", string(pkt.Options[60]))
	}
	// option 93 must be set
	if !pkt.Options.Has(dhcpv4.OptionClientSystemArchitectureType) {
		return errors.New("not a PXE boot request (missing option 93)")
	}
	// option 94 must be set
	if !pkt.Options.Has(dhcpv4.OptionClientNetworkInterfaceIdentifier) {
		return errors.New("not a PXE boot request (missing option 94)")
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
			return errors.New("malformed client GUID (option 97), leading byte must be zero")
		}
	default:
		return errors.New("malformed client GUID (option 97), wrong size")
	}

	return nil
}

// opt43 is completely standard PXE: we tell the PXE client to
// bypass all the boot discovery rubbish that PXE supports,
// and just load a file from TFTP.
func _opt43(reply *dhcpv4.DHCPv4, hw net.HardwareAddr) {

	pxe := dhcpv4.Options{
		// PXE Boot Server Discovery Control - bypass, just boot from filename.
		6: []byte{8}, // or []byte{8}
	}
	// Raspberry PI's need options 9 and 10 of parent option 43.
	// The best way at the moment to figure out if a DHCP request is coming from a Raspberry PI is to
	// check the MAC address. We could reach out to some external server to tell us if the MAC address should
	// use these extra Raspberry PI options but that would require a dependency on some external service and all the trade-offs that
	// come with that. TODO: provide doc link for why these options are needed.
	// https://udger.com/resources/mac-address-vendor-detail?name=raspberry_pi_foundation
	h := strings.ToLower(hw.String())
	if strings.HasPrefix(h, strings.ToLower("B8:27:EB")) ||
		strings.HasPrefix(h, strings.ToLower("DC:A6:32")) ||
		strings.HasPrefix(h, strings.ToLower("E4:5F:01")) {
		// TODO document what these hex strings are and why they are needed.
		// https://www.raspberrypi.org/documentation/computers/raspberry-pi.html#PXE_OPTION43
		// tested with Raspberry Pi 4 using UEFI from here: https://github.com/pftf/RPi4/releases/tag/v1.31
		// all files were served via a tftp server and lived at the top level dir of the tftp server (i.e tftp://server/)
		opt9, _ := hex.DecodeString("00001152617370626572727920506920426f6f74") // "Raspberry Pi Boot"
		opt10, _ := hex.DecodeString("00505845")                                // "PXE"
		pxe[9] = opt9
		pxe[10] = opt10
		fmt.Println("PXE: Raspberry Pi detected, adding options 9 and 10")
	}

	//reply.Options[43] = pxe.ToBytes()
	reply.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionVendorSpecificInformation, pxe.ToBytes()))
}

// processMachine takes a DHCP packet and returns a populated machine.
func _processMachine(pkt *dhcpv4.DHCPv4) (machine, error) {
	mach := machine{}
	// get option 93 ; arch
	fwt := pkt.ClientArch()
	if len(fwt) == 0 {
		return mach, fmt.Errorf("could not determine client architecture")
	}

	// set option 77 from received packet
	mach.uClass = UserClass(string(pkt.GetOneOption(dhcpv4.OptionUserClassInformation)))

	mach.mac = pkt.ClientHWAddr
	// Basic architecture identification, based purely on
	// the PXE architecture option.
	// https://www.rfc-editor.org/errata_search.php?rfc=4578
	switch fwt[0] {
	case 0:
		mach.arch = X86PC
	case 1:
		mach.arch = NecPC98
	case 2:
		mach.arch = EFIItanium
	case 3:
		mach.arch = DecAlpha
	case 4:
		mach.arch = Arcx86
	case 5:
		mach.arch = IntelLeanClient
	case 6:
		mach.arch = EFIIA32
	case 7:
		mach.arch = EFIx8664
	case 8:
		mach.arch = EFIXscale
	case 9:
		mach.arch = EFIBC
	case 10:
		mach.arch = EFIARM
	case 11:
		mach.arch = EFIAARCH64
	case 15:
		mach.arch = EFIx86Http
	case 16:
		mach.arch = EFIx8664Http
	case 18:
		mach.arch = EFIARMHttp
	case 19:
		mach.arch = EFIAARCH64Http
	default:
		return mach, fmt.Errorf("unsupported client firmware type '%d' for %q (please file a bug!)", fwt, mach.mac)
	}

	return mach, nil
}
