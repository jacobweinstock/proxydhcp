//go:build linux
package proxy

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/nclient4"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/insomniacslk/dhcp/iana"
	"github.com/insomniacslk/dhcp/interfaces"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"inet.af/netaddr"
)

// utility function to set up a client and a server instance and run it in
// background. The caller needs to call Server.Close() once finished.
func setUpClientAndServer(t *testing.T, iface net.Interface, handler server4.Handler) (*nclient4.Client, *server4.Server) {
	// strong assumption, I know
	loAddr := net.ParseIP("127.0.0.1")
	saddr := &net.UDPAddr{
		IP:   loAddr,
		Port: 67,
	}
	caddr := net.UDPAddr{
		IP:   loAddr,
		Port: 68,
	}
	s, err := server4.NewServer("", saddr, handler)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		err := s.Serve()
		if err != nil {
			fmt.Println(err)
		}
	}()

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

// defaultLogger is zap logr implementation.
func defaultLogger(level string) logr.Logger {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	switch level {
	case "debug":
		config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	default:
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
	zapLogger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}

	return zapr.NewLogger(zapLogger)
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
		WithLogger(defaultLogger("debug")),
		WithTFTPAddr(ta),
		WithHTTPAddr(ha),
		WithIPXEAddr(ia),
		WithIPXEScript("auto.ipxe"),
		WithUserClass("Tinkerbell"),
	}
	h := NewHandler(context.Background(), opts...)
	c, s := setUpClientAndServer(t, ifaces[0], h.Redirection)
	defer func() {
		s.Close()
	}()

	xid := dhcpv4.TransactionID{0xaa, 0xbb, 0xcc, 0xdd}

	modifiers := []dhcpv4.Modifier{
		dhcpv4.WithTransactionID(xid),
		dhcpv4.WithHwAddr(ifaces[0].HardwareAddr),
		dhcpv4.WithGeneric(dhcpv4.OptionClassIdentifier, []byte("PXEClient:Arch:xxxxx:UNDI:yyyzzz")),
		func(d *dhcpv4.DHCPv4) { d.UpdateOption(dhcpv4.OptClientArch(iana.EFI_X86_64)) },
		dhcpv4.WithGeneric(dhcpv4.OptionClientNetworkInterfaceIdentifier, []byte{1, 2, 1}),
		dhcpv4.WithGeneric(dhcpv4.OptionClientMachineIdentifier, []byte{0, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3, 4, 5, 6, 7, 8}),
	}

	t.Log("HERE")
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	offer, err := c.DiscoverOffer(ctx, modifiers...)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", offer)

	t.Fatal("TODO")
}
