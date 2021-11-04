// Package proxy implements proxydhcp functionality
//
// This was taken from https://github.com/danderson/netboot/blob/master/pixiecore/dhcp.go
// and modified. Contributions to pixiecore would have been preferred but pixiecore
// has not been maintained for some time now.
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
	"github.com/pkg/errors"
	"go.universe.tf/netboot/dhcp4"
	"inet.af/netaddr"
)

// machine describes a device that is requesting a network boot.
type machine struct {
	mac    net.HardwareAddr
	arch   Architecture
	uClass UserClass
}

// Serve proxydhcp requests.
// 1. listen for generic DHCP packets [conn.RecvDHCP()]
// 2. check if the DHCP packet is requesting PXE boot [isPXEPacket(pkt)]
// 3.
func Serve(ctx context.Context, l logr.Logger, conn *dhcp4.Conn, tftpAddr, httpAddr, ipxeAddr, ipxeScript, uClass string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// RecvDHCP is a blocking call
		pkt, intf, err := conn.RecvDHCP()
		if err != nil {
			l.Info("Error receiving DHCP packet", "err", err.Error())
			continue
		}
		if intf == nil {
			l.Info("Received DHCP packet with no interface information (this is a violation of dhcp4.Conn's contract, please file a bug)")
			continue
		}

		go func(pkt *dhcp4.Packet) {
			resp := dhcp4.Packet{
				Options: make(dhcp4.Options),
			}
			switch pkt.Type {
			case dhcp4.MsgDiscover:
				if err = isDiscoverPXEPacket(pkt); err != nil {
					l.Info("Ignoring packet", "hwaddr", pkt.HardwareAddr, "error", err.Error())
					return
				}
				// dhcp discover packets should be answered with a dhcp offer
				resp.Type = dhcp4.MsgOffer

			case dhcp4.MsgRequest:
				if err = isRequestPXEPacket(pkt); err != nil {
					l.Info("Ignoring packet", "hwaddr", pkt.HardwareAddr, "error", err.Error())
					return
				}
				// dhcp request packets should be answered with a dhcp ack
				resp.Type = dhcp4.MsgAck
			default:
				l.Info("Ignoring packet", "hwaddr", pkt.HardwareAddr)
				return
			}

			// TODO add link to intel spec for this needing to be set
			resp, err = opt43(resp, pkt.HardwareAddr)
			if err != nil {
				l.Info("error setting opt 43", "hwaddr", pkt.HardwareAddr, "error", err.Error())
			}

			resp = withGenericHeaders(resp, pkt.TransactionID, pkt.HardwareAddr, pkt.RelayAddr)
			resp = opt60(resp, pxeClient)
			resp = withOpt97(resp, pkt.Options[97])
			resp = withHeaderCiaddr(resp)

			mach, err := processMachine(pkt)
			if err != nil {
				l.Info("Unusable packet", "hwaddr", pkt.HardwareAddr, "error", err.Error(), "mach", mach)
				return
			}

			l.Info("Got valid request to boot", "hwAddr", mach.mac, "arch", mach.arch, "userClass", mach.uClass)

			/*
				bootFileName, found := Defaults[mach.arch]
				if !found {
					bootFileName = fmt.Sprintf(DefaultsHTTP[mach.arch], httpAddr)
					resp = opt60(resp, httpClient)
					ha, _ := url.Parse(httpAddr)
					nextServer, _ := netaddr.ParseIP(ha.Host)
					resp.Options[54] = nextServer.IPAddr().IP
					resp = withHeaderSiaddr(resp, nextServer.IPAddr().IP)
					resp = withHeaderSname(resp, nextServer.String())
					resp = withHeaderBfilename(resp, filepath.Join(ha.Path, bootFileName))
					l.Info("arch was http of some kind", "arch", mach.arch, "userClass", mach.uClass)
				} else {
					ta, _ := url.Parse(tftpAddr)
					nextServer, err := netaddr.ParseIP(ta.Host)
					if err != nil {
						// This error needs to be captured way up the stack in the cli flag validation
						l.Info("error parsing next server", "error", err.Error())
					}
					l.Info("debugging", "nextServer", nextServer)
					resp.Options[54] = nextServer.IPAddr().IP
					resp = withHeaderSiaddr(resp, nextServer.IPAddr().IP)
					resp = withHeaderSname(resp, nextServer.String())
					resp = withHeaderBfilename(resp, filepath.Join(ta.Path, bootFileName))
				}

				if mach.uClass == IPXE || mach.uClass == Tinkerbell || (uClass != "" && mach.uClass == UserClass(uClass)) {
					resp = withHeaderBfilename(resp, ipxeURL)
				}
			*/

			fname, _ := url.Parse(tftpAddr)
			i, _ := netaddr.ParseIP(fname.Host)
			bootFileName, found := Defaults[mach.arch]
			if !found {
				bootFileName = fmt.Sprintf(DefaultsHTTP[mach.arch], httpAddr)
				l.Info("debugging", "bootfilename", bootFileName)
				resp = opt60(resp, httpClient)
				i, _ = netaddr.ParseIP(httpAddr)
				l.Info("arch was http of some kind", mach.arch, "userClass", mach.uClass)
			}

			resp.Options[54] = i.IPAddr().IP
			resp = withHeaderSiaddr(resp, i.IPAddr().IP)
			resp = withHeaderSname(resp, i.String())

			// If a machine is in an ipxe boot loop, it is likely to be that we arent matching on IPXE or Tinkerbell
			if mach.uClass == IPXE || mach.uClass == Tinkerbell || (uClass != "" && mach.uClass == UserClass(uClass)) {
				resp = withHeaderBfilename(resp, fmt.Sprintf("%s/%s/%s", ipxeAddr, mach.mac.String(), ipxeScript))
			} else {
				resp = withHeaderBfilename(resp, filepath.Join(fname.Path, bootFileName))
			}
			l.Info("debugging", "bootfilename", bootFileName)

			if err = conn.SendDHCP(&resp, intf); err != nil {
				l.Info("Failed to send ProxyDHCP offer", "hwaddr", pkt.HardwareAddr, "error", err.Error())
				return
			}
			l.Info("Sent ProxyDHCP msg", "msg", fmt.Sprintf("%+v", resp), "struct", resp)
		}(pkt)
	}
}

// processMachine takes a DHCP packet and returns a populated machine.
func processMachine(pkt *dhcp4.Packet) (machine, error) {
	mach := machine{}
	fwt, err := pkt.Options.Uint16(93)
	if err != nil {
		return mach, fmt.Errorf("malformed DHCP option 93 (required for PXE): %w", err)
	}
	if userClass, err := pkt.Options.String(77); err == nil {
		mach.uClass = UserClass(userClass)
	}
	mach.mac = pkt.HardwareAddr
	// Basic architecture identification, based purely on
	// the PXE architecture option.
	// https://www.rfc-editor.org/errata_search.php?rfc=4578
	switch fwt {
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

// withHeaderCiaddr adds the siaddr (IP address of next server) dhcp packet header to a given packet pkt.
// see https://datatracker.ietf.org/doc/html/rfc2131#section-2
func withHeaderSiaddr(pkt dhcp4.Packet, siaddr net.IP) dhcp4.Packet {
	pkt.ServerAddr = siaddr
	return pkt
}

func withHeaderCiaddr(pkt dhcp4.Packet) dhcp4.Packet {
	pkt.ServerAddr = net.IP{0, 0, 0, 0} // does it need tobe null?
	return pkt
}

func withHeaderSname(pkt dhcp4.Packet, sn string) dhcp4.Packet {
	pkt.BootServerName = sn
	return pkt
}

func withHeaderBfilename(pkt dhcp4.Packet, bf string) dhcp4.Packet {
	pkt.BootFilename = bf
	return pkt
}

// withGenericHeaders updates a dhcp packet with the required dhcp headers.
func withGenericHeaders(pkt dhcp4.Packet, tID []byte, mac net.HardwareAddr, rAddr net.IP) dhcp4.Packet {
	pkt.TransactionID = tID
	pkt.Broadcast = true
	pkt.HardwareAddr = mac
	pkt.RelayAddr = rAddr

	return pkt
}

func withOpt97(pkt dhcp4.Packet, guid []byte) dhcp4.Packet {
	if guid != nil {
		pkt.Options[97] = guid
	}

	return pkt
}

func opt60(pkt dhcp4.Packet, c clientType) dhcp4.Packet {
	// The PXE spec says the server should identify itself as a PXEClient or HTTPClient
	pkt.Options[dhcp4.OptVendorIdentifier] = []byte(c)

	return pkt
}

var (
	Defaults = map[Architecture]string{
		X86PC:           "undionly.kpxe",
		NecPC98:         "undionly.kpxe",
		EFIItanium:      "undionly.kpxe",
		DecAlpha:        "undionly.kpxe",
		Arcx86:          "undionly.kpxe",
		IntelLeanClient: "undionly.kpxe",
		EFIIA32:         "ipxe.efi",
		EFIx8664:        "ipxe.efi",
		EFIXscale:       "ipxe.efi",
		EFIBC:           "ipxe.efi",
		EFIARM:          "snp.efi",
		EFIAARCH64:      "snp.efi",
	}
	DefaultsHTTP = map[Architecture]string{
		EFIx86Http:     "http://%v/ipxe.efi",
		EFIx8664Http:   "http://%v/ipxe.efi",
		EFIARMHttp:     "http://%v/snp.efi",
		EFIAARCH64Http: "http://%v/snp.efi",
	}
)

// opt43 is completely standard PXE: we tell the PXE client to
// bypass all the boot discovery rubbish that PXE supports,
// and just load a file from TFTP.
func opt43(msg dhcp4.Packet, m net.HardwareAddr) (dhcp4.Packet, error) {
	pxe := dhcp4.Options{
		// PXE Boot Server Discovery Control - bypass, just boot from filename.
		6: []byte{8}, // or []byte{8}
	}
	// Raspberry PI's need options 9 and 10 of parent option 43.
	// The best way at the moment to figure out if a DHCP request is coming from a Raspberry PI is to
	// check the MAC address. We could reach out to some external server to tell us if the MAC address should
	// use these extra Raspberry PI options but that would require a dependency on some external service and all the trade-offs that
	// come with that. TODO: provide doc link for why these options are needed.
	// https://udger.com/resources/mac-address-vendor-detail?name=raspberry_pi_foundation
	if strings.HasPrefix(strings.ToLower(m.String()), strings.ToLower("B8:27:EB")) ||
		strings.HasPrefix(strings.ToLower(m.String()), strings.ToLower("DC:A6:32")) ||
		strings.HasPrefix(strings.ToLower(m.String()), strings.ToLower("E4:5F:01")) {
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

	bs, err := pxe.Marshal()
	if err != nil {
		return dhcp4.Packet{}, fmt.Errorf("failed to serialize PXE Boot Server Discovery Control: %w", err)
	}
	msg.Options[43] = bs

	return msg, nil
}

/*
func interfaceIP(intf *net.Interface) net.IP {
	addrs, err := intf.Addrs()
	if err != nil {
		return nil
	}

	// Try to find an IPv4 address to use, in the following order:
	// global unicast (includes rfc1918), link-local unicast,
	// loopback.
	fs := [](func(net.IP) bool){
		net.IP.IsGlobalUnicast,
		net.IP.IsLinkLocalUnicast,
		net.IP.IsLoopback,
	}
	for _, f := range fs {
		for _, a := range addrs {
			ipaddr, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipaddr.IP.To4()
			if ip == nil {
				continue
			}
			if f(ip) {
				return ip
			}
		}
	}

	return nil
}
*/

// isDiscoverPXEPacket determines if the DHCP packet meets qualifications of a being a PXE enabled client.
// 1. is a DHCP discovery message type
// 2. option 93 is set
// 3. option 94 is set
// 4. option 97 is correct length.
// 5. option 60 is set with this format: "PXEClient:Arch:xxxxx:UNDI:yyyzzz" or "HTTPClient:Arch:xxxxx:UNDI:yyyzzz"
// 6. option 55 is set; only warn if not set
// 7. options 128-135 are set; only warn if not set.
func isDiscoverPXEPacket(pkt *dhcp4.Packet) error {
	// should only be a dhcp discover because a request packet has different requirements
	if pkt.Type != dhcp4.MsgDiscover {
		return fmt.Errorf("DHCP message type is %s, must be %s", pkt.Type, dhcp4.MsgDiscover)
	}
	// option 55 must be set
	if pkt.Options[55] == nil {
		// just warn for the moment because we don't actually do anything with this option
		fmt.Println("warning: missing option 55")
	}
	// option 60 must be set
	if opt60 := pkt.Options[60]; opt60 == nil {
		return errors.New("not a PXE boot request (missing option 60)")
	}
	// option 60 must start with PXEClient
	if !strings.HasPrefix(string(pkt.Options[60]), "PXEClient") && !strings.HasPrefix(string(pkt.Options[60]), "HTTPClient") {
		return fmt.Errorf("not a PXE boot request (option 60 does not start with PXEClient: %v)", string(pkt.Options[60]))
	}
	// option 93 must be set
	if pkt.Options[93] == nil {
		return errors.New("not a PXE boot request (missing option 93)")
	}
	// option 93 must be set
	if pkt.Options[94] == nil {
		return errors.New("not a PXE boot request (missing option 94)")
	}
	// option 97 must be have correct length or not be set
	guid := pkt.Options[97]
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
	for i := 128; i <= 135; i++ {
		v := pkt.Options[dhcp4.Option(i)]
		if v == nil {
			fmt.Printf("warning: missing option %d\n", i)
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
func isRequestPXEPacket(pkt *dhcp4.Packet) error {
	// should only be a dhcp request messsage type because a discover message type has different requirements
	if pkt.Type != dhcp4.MsgRequest {
		return fmt.Errorf("DHCP message type is %s, must be %s", pkt.Type, dhcp4.MsgRequest)
	}
	// option 60 must be set
	if opt60 := pkt.Options[60]; opt60 == nil {
		return errors.New("not a PXE boot request (missing option 60)")
	}
	// option 60 must start with PXEClient
	if !strings.HasPrefix(string(pkt.Options[60]), string(pxeClient)) && !strings.HasPrefix(string(pkt.Options[60]), string(httpClient)) {
		return errors.New("not a PXE boot request (option 60 does not start with PXEClient)")
	}
	// option 93 must be set
	if pkt.Options[93] == nil {
		return errors.New("not a PXE boot request (missing option 93)")
	}
	// option 93 must be set
	if pkt.Options[94] == nil {
		return errors.New("not a PXE boot request (missing option 94)")
	}
	// option 97 must be have correct length or not be set
	guid := pkt.Options[97]
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
