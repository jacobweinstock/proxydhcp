// Package proxy implements proxydhcp functionality
//
// This was taken from https://github.com/danderson/netboot/blob/master/pixiecore/dhcp.go
// and modified. Contributions to pixiecore would have been preferred but pixiecore
// has not been maintained for some time now.
package proxy

import (
	"context"
	"fmt"
	"net"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"go.universe.tf/netboot/dhcp4"
	"inet.af/netaddr"
)

// Locator interface for getting options 66 and 67 - ipxe binary and script locations.
type Locator interface {
	// Locate takes in context, hardware (mac) address, User-Class (User Class Information) - option 77, Client System (Client System Architecture) - option 93
	// and returns the location to an ipxe binary or script.
	// It returns Server-Name (TFTP Server Name) - option 66, the Bootfile-Name (Boot File Name) - option 67.
	Locate(context.Context, net.HardwareAddr, UserClass, Architecture) (FileName string, ServerName string, err error)
}

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
func Serve(ctx context.Context, l logr.Logger, conn *dhcp4.Conn, loc Locator) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// RecvDHCP is a blocking call
		pkt, intf, err := conn.RecvDHCP()
		if err != nil {
			l.V(0).Info("Error receiving DHCP packet", "err", err.Error())
			continue
		}
		if intf == nil {
			l.V(0).Info("Received DHCP packet with no interface information (this is a violation of dhcp4.Conn's contract, please file a bug)")
			continue
		}

		go func(pkt *dhcp4.Packet) {
			if err = isPXEPacket(pkt); err != nil {
				l.V(0).Info("Ignoring packet", "hwaddr", pkt.HardwareAddr, "error", err.Error())
				return
			}
			mach, err := processMachine(pkt)
			if err != nil {
				l.V(0).Info("Unusable packet", "hwaddr", pkt.HardwareAddr, "error", err.Error())
				return
			}

			l.V(0).Info("Got valid request to boot", "hwAddr", mach.mac, "arch", mach.arch)

			m, err := createMSG(ctx, pkt, mach)
			if err != nil {
				l.V(0).Error(err, "Failed to construct ProxyDHCP offer", "hwaddr", pkt.HardwareAddr)
				return
			}
			switch pkt.Type {
			case dhcp4.MsgDiscover:
				m.Type = dhcp4.MsgOffer
			case dhcp4.MsgRequest:
				m.Type = dhcp4.MsgAck
			default:
			}

			msg, err := bootOpts(ctx, *m, mach, loc, interfaceIP(intf).String())
			if err != nil {
				l.V(0).Error(err, "Failed to construct ProxyDHCP offer", "hwaddr", pkt.HardwareAddr)
				return
			}

			if err = conn.SendDHCP(msg, intf); err != nil {
				l.V(0).Info("Failed to send ProxyDHCP offer", "hwaddr", pkt.HardwareAddr, "error", err.Error())
				return
			}
			l.V(0).Info("Sent ProxyDHCP msg", "msg", fmt.Sprintf("%+v", msg), "struct", msg)
		}(pkt)
	}
}

// isPXEPacket determines if the packet meets qualifications of a PXE request
// 1. is a DHCP discovery or request packet
// 2. option 93 is set
// 3. option 97 is correct length.
func isPXEPacket(pkt *dhcp4.Packet) error {
	// should be a dhcp discover or request packet
	switch pkt.Type {
	case dhcp4.MsgDiscover, dhcp4.MsgRequest:
		// good
	default:
		return fmt.Errorf("packet is %s, not %s or %s", pkt.Type, dhcp4.MsgDiscover, dhcp4.MsgRequest)
	}

	// option 93 must be set
	if pkt.Options[93] == nil {
		return errors.New("not a PXE boot request (missing option 93)")
	}
	// option 97 must be have correct length
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

// processMachine reads a dhcp packet and populates a machine struct.
func processMachine(pkt *dhcp4.Packet) (mach machine, err error) {
	fwt, err := pkt.Options.Uint16(93)
	if err != nil {
		return mach, fmt.Errorf("malformed DHCP option 93 (required for PXE): %w", err)
	}
	// Basic architecture identification, based purely on
	// the PXE architecture option.
	// https://www.rfc-editor.org/errata_search.php?rfc=4578
	switch fwt {
	case 0:
		mach.arch = X86PC
	case 1:
		mach.arch = NecPC98
	case 2:
		mach.arch = EfiItanium
	case 3:
		mach.arch = DecAlpha
	case 4:
		mach.arch = Arcx86
	case 5:
		mach.arch = IntelLeanClient
	case 6:
		mach.arch = EfiIA32
	case 7:
		mach.arch = Efix8664
	case 8:
		mach.arch = EfiXscale
	case 9:
		mach.arch = EfiBC
	default:
		return mach, fmt.Errorf("unsupported client firmware type '%d' (please file a bug!)", fwt)
	}

	if userClass, err := pkt.Options.String(77); err == nil {
		mach.uClass = UserClass(userClass)
	}
	mach.mac = pkt.HardwareAddr
	return mach, nil
}

// createMSG returns a dhcp packet.
func createMSG(_ context.Context, pkt *dhcp4.Packet, mach machine) (*dhcp4.Packet, error) {
	resp := &dhcp4.Packet{
		Type:          dhcp4.MsgOffer,
		TransactionID: pkt.TransactionID,
		Broadcast:     true,
		HardwareAddr:  mach.mac,
		RelayAddr:     pkt.RelayAddr,
		Options:       make(dhcp4.Options),
	}

	// says the server should identify itself as a PXEClient vendor
	// type, even though it's a server. Strange.
	resp.Options[dhcp4.OptVendorIdentifier] = []byte("PXEClient")
	if pkt.Options[97] != nil {
		resp.Options[97] = pkt.Options[97]
	}
	// This is completely standard PXE: we tell the PXE client to
	// bypass all the boot discovery rubbish that PXE supports,
	// and just load a file from TFTP.
	pxe := dhcp4.Options{
		// PXE Boot Server Discovery Control - bypass, just boot from filename.
		6: []byte{8},
	}
	bs, err := pxe.Marshal()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize PXE Boot Server Discovery Control: %w", err)
	}
	resp.Options[43] = bs

	return resp, nil
}

// bootOpts updates a DHCP packet with values for options 54, 66, & 67 set.
func bootOpts(ctx context.Context, msg dhcp4.Packet, mach machine, loc Locator, serverIP string) (*dhcp4.Packet, error) {
	var err error
	msg.BootFilename, msg.BootServerName, err = loc.Locate(ctx, mach.mac, mach.uClass, mach.arch)
	if err != nil {
		return nil, errors.Wrap(err, "failed to locate boot file")
	}
	if msg.BootServerName != "" {
		i, _ := netaddr.ParseIP(msg.BootServerName)
		if i.Is4() {
			msg.ServerAddr = i.IPAddr().IP // this needs match the BootServerName where ipxe binaries or scripts are hosted
			msg.Options[dhcp4.OptServerIdentifier] = i.IPAddr().IP
		}
	} else {
		a, _ := netaddr.ParseIP(serverIP) // if the Bootfilename is a full URL, then the ServerAddr doesn't matter. It just can't be 0.0.0.0 or nil.
		msg.ServerAddr = a.IPAddr().IP
	}
	return &msg, nil
}

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
