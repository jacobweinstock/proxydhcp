package proxy

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"path/filepath"

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
				resp = withHeaderBfilename(resp, filepath.Join(fname.Path, bootFileName))
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
