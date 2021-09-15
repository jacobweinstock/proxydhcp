package app

import (
	"context"
	"net"
	"net/url"

	"github.com/jacobweinstock/proxyDHCP/proxy"
)

type Default struct {
	URL *url.URL
}

func (d Default) Locate(_ context.Context, _ net.HardwareAddr, uc proxy.UserClass, arch proxy.Architecture) (string, string, error) {
	var bootfilename, bootservername string
	switch arch {
	case proxy.X86PC:
		bootfilename = "undionly.kpxe"
		bootservername = d.URL.Host
	case proxy.EfiIA32, proxy.Efix8664, proxy.EfiBC:
		bootfilename = "ipxe.efi"
		bootservername = d.URL.Host
	default:
		bootfilename = "/unsupported"
	}
	switch uc {
	case proxy.IPXE, proxy.Tinkerbell:
		bootfilename = "http://boot.netboot.xyz"
		bootservername = ""
	default:
	}

	return bootfilename, bootservername, nil
}
