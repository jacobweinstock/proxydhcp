package proxy

import (
	"context"
	"fmt"
	"net"

	"github.com/go-logr/logr"
	"go.universe.tf/netboot/dhcp4"
	"golang.org/x/net/ipv4"
	"inet.af/netaddr"
)

// ServeBoot handles dhcp request message types.
// must listen on port 4011.
// 1. listen for generic DHCP packets [conn.RecvDHCP()]
// 2. check if the DHCP packet is requesting PXE boot [isPXEPacket(pkt)]
// 3.
func ServeBoot(ctx context.Context, l logr.Logger, conn net.PacketConn, tftpAddr, httpAddr, ipxeURL, uClass string) {
	listener := ipv4.NewPacketConn(conn)
	if err := listener.SetControlMessage(ipv4.FlagInterface, true); err != nil {
		panic(fmt.Errorf("Couldn't get interface metadata on PXE port: %s", err))
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
			l.V(0).Info("Error Receiving packet:", err)
			continue
		}
		pkt, err := dhcp4.Unmarshal(buf[:n])
		if err != nil {
			l.V(0).Info("Packet from %s is not a DHCP packet: %s", addr, err)
			continue
		}

		go func(pkt *dhcp4.Packet) {
			resp := dhcp4.Packet{
				Options: make(dhcp4.Options),
			}
			switch pkt.Type {
			case dhcp4.MsgRequest:
				if err = isRequestPXEPacket(pkt); err != nil {
					l.V(0).Info("Ignoring packet", "hwaddr", pkt.HardwareAddr, "error", err.Error())
					return
				}
				// dhcp request packets should be answered with a dhcp ack
				resp.Type = dhcp4.MsgAck
			default:
				l.V(0).Info("Ignoring packet", "hwaddr", pkt.HardwareAddr)
				return
			}

			// TODO add link to intel spec for this needing to be set
			resp, err = setOpt43(resp, pkt.HardwareAddr)
			if err != nil {
				l.V(0).Info("error setting opt 43", "hwaddr", pkt.HardwareAddr, "error", err.Error())
			}

			resp = withGenericHeaders(resp, pkt.TransactionID, pkt.HardwareAddr, pkt.RelayAddr)
			resp = setOpt60(resp, pxeClient)
			resp = withOpt97(resp, pkt.Options[97])
			resp = withHeaderCiaddr(resp)

			mach, err := processMachine(pkt)
			if err != nil {
				l.V(0).Info("Unusable packet", "hwaddr", pkt.HardwareAddr, "error", err.Error(), "mach", mach)
				return
			}

			l.V(0).Info("Got valid request to boot", "hwAddr", mach.mac, "arch", mach.arch, "userClass", mach.uClass)

			i, _ := netaddr.ParseIP(tftpAddr)
			bootFileName, found := defaults[mach.arch]
			if !found {
				bootFileName = fmt.Sprintf(defaultsHTTP[mach.arch], httpAddr)
				resp = setOpt60(resp, httpClient)
				i, _ = netaddr.ParseIP(httpAddr)
				l.V(0).Info("arch was http of some kind", mach.arch, "userClass", mach.uClass)
			}

			resp.Options[54] = i.IPAddr().IP
			resp = withHeaderSiaddr(resp, i.IPAddr().IP)
			resp = withHeaderSname(resp, i.String())

			if mach.uClass == IPXE || mach.uClass == Tinkerbell || (uClass != "" && mach.uClass == UserClass(uClass)) {
				resp = withHeaderBfilename(resp, ipxeURL)
			} else {
				resp = withHeaderBfilename(resp, bootFileName)
			}

			bs, err := resp.Marshal()
			if err != nil {
				l.V(0).Info("Failed to marshal PXE offer for %s (%s): %s", pkt.HardwareAddr, addr, err)
				return
			}

			if _, err := listener.WriteTo(bs, &ipv4.ControlMessage{
				IfIndex: msg.IfIndex,
			}, addr); err != nil {
				l.V(0).Info("PXE", "Failed to send PXE response", "pkt.HardwareAddr", pkt.HardwareAddr, "addr", addr, "err", err)
			}
			l.V(0).Info("Sent ProxyDHCP msg", "msg", fmt.Sprintf("%+v", resp), "struct", resp)
		}(pkt)
	}
}
