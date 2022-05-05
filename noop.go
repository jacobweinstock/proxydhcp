package proxydhcp

import (
	"log"
	"net"
	"os"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

type Noop struct {
	Log logr.Logger
}

func (n *Noop) Handle(conn net.PacketConn, peer net.Addr, m *dhcpv4.DHCPv4) {
	msg := "no handler specified. please specify a handler"
	if n.Log.GetSink() == nil {
		stdr.New(log.New(os.Stdout, "", log.Lshortfile)).Info(msg)
	} else {
		n.Log.Info(msg)
	}
}
