package proxy

import (
	"strings"

	"github.com/hashicorp/go-multierror"
	"go.universe.tf/netboot/dhcp4"
)

// NewListener is a place holder for proxydhcp being a proper subcommand
// its goal is to serve proxydhcp requests.
func NewListener(addr string) (*dhcp4.Conn, error) {
	conn, err := dhcp4.NewConn(formatAddr(addr))
	if err != nil {
		var serr error
		conn, serr = dhcp4.NewSnooperConn(formatAddr(addr))
		if err != nil {
			return nil, multierror.Append(err, serr)
		}
	}

	return conn, nil
}

// formatAddr will add 0.0.0.0 to a host:port combo that is without a host i.e. ":67".
func formatAddr(s string) string {
	if strings.HasPrefix(s, ":") {
		return "0.0.0.0" + s
	}
	return s
}
