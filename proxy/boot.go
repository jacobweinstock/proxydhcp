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
	"golang.org/x/net/ipv4"
	"inet.af/netaddr"
)

// ServeBoot handles dhcp request message types.
// must listen on port 4011.
// 1. listen for generic DHCP packets [conn.RecvDHCP()]
// 2. check if the DHCP packet is requesting PXE boot [isPXEPacket(pkt)]
// 3.
func ServeBoot(ctx context.Context, l logr.Logger, conn net.PacketConn, tftpAddr, httpAddr, ipxeAddr, ipxeScript, uClass string) {
	listener := ipv4.NewPacketConn(conn)
	if err := listener.SetControlMessage(ipv4.FlagInterface, true); err != nil {
		panic(fmt.Errorf("couldn't get interface metadata on PXE port: %w", err))
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		buf := make([]byte, 1024)
		n, msg, addr, err := listener.ReadFrom(buf)
		if err != nil {
			l.Info("Error Receiving packet:", "err", err)
			continue
		}
		pkt, err := dhcp4.Unmarshal(buf[:n])
		if err != nil {
			l.Info("Packet is not a DHCP packet", "addr", addr, "err", err)
			continue
		}
		l.Info("Received DHCP packet", "addr", addr, "msg", msg)
		go func(pkt *dhcp4.Packet) {
			resp := dhcp4.Packet{
				Options: make(dhcp4.Options),
			}
			switch pkt.Type {
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
				bootFileName, found := defaults[mach.arch]
				if !found {
					bootFileName = fmt.Sprintf(defaultsHTTP[mach.arch], httpAddr)
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
					nextServer, _ := netaddr.ParseIP(ta.Host)
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
				resp = opt60(resp, httpClient)
				i, _ = netaddr.ParseIP(httpAddr)
				l.Info("arch was http of some kind", mach.arch, "userClass", mach.uClass)
			}

			resp.Options[54] = i.IPAddr().IP
			resp = withHeaderSiaddr(resp, i.IPAddr().IP)
			resp = withHeaderSname(resp, i.String())

			if mach.uClass == IPXE || mach.uClass == Tinkerbell || (uClass != "" && mach.uClass == UserClass(uClass)) {
				resp = withHeaderBfilename(resp, fmt.Sprintf("%s/%s/%s", ipxeAddr, mach.mac.String(), ipxeScript))
			} else {
				resp = withHeaderBfilename(resp, filepath.Join(fname.Path, mach.mac.String(), bootFileName))
			}

			bs, err := resp.Marshal()
			if err != nil {
				l.Info("Failed to marshal PXE offer for %s (%s): %s", pkt.HardwareAddr, addr, err)
				return
			}

			if _, err := listener.WriteTo(bs, &ipv4.ControlMessage{
				IfIndex: msg.IfIndex,
			}, addr); err != nil {
				l.Info("PXE", "Failed to send PXE response", "pkt.HardwareAddr", pkt.HardwareAddr, "addr", addr, "err", err)
			}
			l.Info("Sent ProxyDHCP msg", "msg", fmt.Sprintf("%+v", resp), "struct", resp)
		}(pkt)
	}
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
