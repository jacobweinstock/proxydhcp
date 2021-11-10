//go:build linux

package proxy

import (
	"context"
	"errors"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/insomniacslk/dhcp/iana"
	"github.com/insomniacslk/dhcp/interfaces"
	"inet.af/netaddr"
)

// utility function to set up a client and a server instance and run it in
// background. The caller needs to call Server.Close() once finished.
func setUpClientAndServer(t *testing.T, iface net.Interface, handler server4.Handler) (*nclient4.Client, *server4.Server) {
	t.Helper()
	loAddr := net.ParseIP("127.0.0.1")
	saddr := &net.UDPAddr{
		IP:   loAddr,
		Port: 6767,
	}
	caddr := net.UDPAddr{
		IP:   loAddr,
		Port: 6868,
	}
	s, err := server4.NewServer("", saddr, handler)
	if err != nil {
		t.Fatal(err)
	}

	clientConn, err := server4.NewIPv4UDPConn("", &caddr)
	if err != nil {
		t.Fatal(err)
	}

	c, err := nclient4.NewWithConn(clientConn, iface.HardwareAddr, nclient4.WithServerAddr(saddr))
	if err != nil {
		t.Fatal(err)
	}
	return c, s
}

func TestRedirection(t *testing.T) {
	ifaces, err := interfaces.GetLoopbackInterfaces()
	if err != nil {
		t.Fatal(err)
	}
	// lo has a HardwareAddr of "nil". The client will drop all packets
	// that don't match the HWAddr of the client interface.
	hwaddr := net.HardwareAddr{1, 2, 3, 4, 5, 6}
	ifaces[0].HardwareAddr = hwaddr
	ta, err := netaddr.ParseIPPort("127.0.0.1:69")
	if err != nil {
		t.Fatal(err)
	}
	ha, err := netaddr.ParseIPPort("127.0.0.1:80")
	if err != nil {
		t.Fatal(err)
	}
	ia, err := url.Parse("http://127.0.0.1/auto.ipxe")
	if err != nil {
		t.Fatal(err)
	}
	opts := []Option{
		WithLogger(logr.Discard()),
		WithTFTPAddr(ta),
		WithHTTPAddr(ha),
		WithIPXEAddr(ia),
		WithIPXEScript("auto.ipxe"),
		WithUserClass("Tinkerbell"),
	}
	h := NewHandler(context.Background(), opts...)
	c, s := setUpClientAndServer(t, ifaces[0], h.Redirection)
	go func() {
		err := s.Serve()
		if err != nil {
			t.Log(err)
		}
	}()
	defer func() {
		s.Close()
	}()

	xid := dhcpv4.TransactionID{0xaa, 0xbb, 0xcc, 0xdd}
	wantMods := []dhcpv4.Modifier{
		dhcpv4.WithTransactionID(xid),
		dhcpv4.WithHwAddr(ifaces[0].HardwareAddr),
		dhcpv4.WithGeneric(dhcpv4.OptionClassIdentifier, []byte("PXEClient")),
		dhcpv4.WithMessageType(dhcpv4.MessageTypeOffer),
		dhcpv4.WithGeneric(dhcpv4.OptionClientMachineIdentifier, []byte{0, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3, 4, 5, 6, 7, 8}),
		dhcpv4.WithGeneric(dhcpv4.OptionVendorSpecificInformation, dhcpv4.Options{6: []byte{8}}.ToBytes()),
		dhcpv4.WithGeneric(dhcpv4.OptionServerIdentifier, net.IP{127, 0, 0, 1}),
	}
	want, _ := dhcpv4.New(wantMods...)
	want.BootFileName = "01:02:03:04:05:06/ipxe.efi"
	want.ServerHostName = "127.0.0.1"
	want.ServerIPAddr = net.IP{127, 0, 0, 1}
	want.OpCode = dhcpv4.OpcodeBootReply
	want.Flags = 32768
	tests := []struct {
		name string
		mods []dhcpv4.Modifier
		want *dhcpv4.DHCPv4
	}{
		{
			name: "failure DHCP option 60 not set",
			mods: []dhcpv4.Modifier{
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover))
				},
			},
		},
		{
			name: "failure invalid optCode",
			mods: []dhcpv4.Modifier{
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover))
					d.OpCode = dhcpv4.OpcodeBootReply
				},
			},
		},
		{
			name: "failure unknown arch",
			mods: []dhcpv4.Modifier{
				dhcpv4.WithMessageType(dhcpv4.MessageTypeRequest),
				dhcpv4.WithTransactionID(xid),
				dhcpv4.WithHwAddr(ifaces[0].HardwareAddr),
				dhcpv4.WithGeneric(dhcpv4.OptionClassIdentifier, []byte("PXEClient:Arch:xxxxx:UNDI:yyyzzz")),
				dhcpv4.WithGeneric(dhcpv4.OptionClientNetworkInterfaceIdentifier, []byte{1, 2, 1}),
				dhcpv4.WithGeneric(dhcpv4.OptionClientMachineIdentifier, []byte{0, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3, 4, 5, 6, 7, 8}),
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptClientArch(37))
				},
			},
		},
		{
			name: "success",
			mods: []dhcpv4.Modifier{
				dhcpv4.WithMessageType(dhcpv4.MessageTypeDiscover),
				dhcpv4.WithTransactionID(xid),
				dhcpv4.WithHwAddr(ifaces[0].HardwareAddr),
				dhcpv4.WithGeneric(dhcpv4.OptionClassIdentifier, []byte("PXEClient:Arch:xxxxx:UNDI:yyyzzz")),
				dhcpv4.WithGeneric(dhcpv4.OptionClientNetworkInterfaceIdentifier, []byte{1, 2, 1}),
				dhcpv4.WithGeneric(dhcpv4.OptionClientMachineIdentifier, []byte{0, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3, 4, 5, 6, 7, 8}),
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptClientArch(iana.EFI_X86_64))
				},
			},
			want: want,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var cancel context.CancelFunc
			if tt.want != nil {
				ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
			} else {
				ctx, cancel = context.WithTimeout(ctx, 1*time.Second)
			}
			defer cancel()

			got, err := c.DiscoverOffer(ctx, tt.mods...)
			if tt.want == nil && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if tt.want != nil {
				if diff := cmp.Diff(got, tt.want); diff != "" {
					t.Fatalf(diff)
				}
			}
		})
	}
}
