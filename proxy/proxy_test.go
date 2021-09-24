package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/google/go-cmp/cmp"
	"github.com/libp2p/go-reuseport"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.universe.tf/netboot/dhcp4"
	"golang.org/x/sync/errgroup"
)

// https://github.com/danderson/netboot/blob/bdaec9d82638460bf166fb98bdc6d97331d7bd80/dhcp4/testdata/dhcp.parsed

// defaultLogger is zap logr implementation.
func defaultLogger() logr.Logger {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	zapLogger, err := config.Build()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}

	return zapr.NewLogger(zapLogger)
}

type testLocator struct {
	ip string
}

func (t testLocator) Locate(_ context.Context, _ net.HardwareAddr, uc UserClass, arch Architecture) (string, string, error) {
	var bootfilename, bootservername string
	switch arch {
	case X86PC:
		bootfilename = "undionly.kpxe"
		bootservername = t.ip
	case EFIIA32, EFIx8664, EFIBC:
		bootfilename = "ipxe.efi"
		bootservername = t.ip
	default:
		bootfilename = "/unsupported"
	}
	switch uc {
	case IPXE, Tinkerbell:
		bootfilename = "http://boot.netboot.xyz"
		bootservername = ""
	default:
	}

	return bootfilename, bootservername, nil
}

func TestServe(t *testing.T) {
	tests := map[string]struct {
		input string
		want  error
	}{
		"context canceled": {input: "127.0.0.1:60656", want: context.Canceled},
	}
	tl := testLocator{ip: "127.0.0.1"}

	logger := defaultLogger()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			conn, err := dhcp4.NewConn(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			ctx, cancel := context.WithCancel(context.Background())
			g, ctx := errgroup.WithContext(ctx)
			g.Go(func() error {
				Serve(ctx, logger, conn)
				return ctx.Err()
			})
			// send DHCP request
			sendPacket(conn)
			if errors.Is(tc.want, context.Canceled) {
				conn.Close()
				cancel()
			}
			got := g.Wait()
			if !errors.Is(got, tc.want) {
				conn.Close()
				cancel()
				t.Fatalf("expected error of type %T, got: %T", tc.want, got)
			}
			conn.Close()
			cancel()
		})
		// t.Fatal()
	}
}

func sendPacket(_ *dhcp4.Conn) {
	con, err := reuseport.Dial("udp4", "127.0.0.1:35689", "127.0.0.1:60656")
	if err != nil {
		fmt.Println("1", err)
		return
	}

	mac, err := net.ParseMAC("ce:e7:7b:ef:45:f7")
	if err != nil {
		fmt.Println("2", err)
		return
	}
	opts := make(dhcp4.Options)
	var opt93 dhcp4.Option = 93
	opts[opt93] = []byte{0x0, 0x0}
	var opt77 dhcp4.Option = 77
	opts[opt77] = []byte("iPXE")
	p := &dhcp4.Packet{
		Type:          dhcp4.MsgDiscover,
		TransactionID: []byte("1234"),
		Broadcast:     true,
		HardwareAddr:  mac,
		Options:       opts,
	}

	bs, err := p.Marshal()
	if err != nil {
		fmt.Println("3", err)
		return
	}

	recPkt := make(chan *dhcp4.Packet)
	go func() {
		con, err := dhcp4.NewConn("")
		if err != nil {
			fmt.Println("err", err)
			return
		}
		/*
			pc, err := reuseport.Dial("udp4", "192.168.2.225:35689", "")
			if err != nil {
				fmt.Println("45 err", err)
				return
			}
			defer pc.Close()
		*/
		for {
			// var buf []byte
			// _, err := pc.Read(buf)
			pkt, _, err := con.RecvDHCP()
			if err == nil {
				// pkt, err := dhcp4.Unmarshal(buf[:])
				// if err == nil {
				if pkt.Type == dhcp4.MsgOffer {
					recPkt <- pkt
					return
				}
				// }
			} else {
				fmt.Println("err", err)
			}
		}
	}()

	con.Write(bs)
	// s.Write(bs)
	select {
	case <-time.After(time.Second * 2):
		close(recPkt)
		return
	case pkt := <-recPkt:
		fmt.Printf("Reply: %+v\n", pkt)
	}
}

func opts(num int) dhcp4.Options {
	opts := dhcp4.Options{93: {0x0, 0x0}}
	switch num {
	case 1:
	case 2:
		opts[97] = []byte{0x0, 0x0, 0x2, 0x0, 0x3, 0x0, 0x4, 0x0, 0x5, 0x0, 0x6, 0x0, 0x7, 0x0, 0x8, 0x0, 0x9}
	case 4:
		opts[97] = []byte{0x2, 0x0, 0x2, 0x0, 0x3, 0x0, 0x4, 0x0, 0x5, 0x0, 0x6, 0x0, 0x7, 0x0, 0x8, 0x0, 0x9}
	case 5:
		opts[97] = []byte{0x2, 0x0, 0x2}
	default:
		opts = make(dhcp4.Options)
	}

	return opts
}

func TestIsPXEPacket(t *testing.T) {
	tests := map[string]struct {
		input *dhcp4.Packet
		want  error
	}{
		"success, len(opt 97) == 0":             {input: &dhcp4.Packet{Type: dhcp4.MsgDiscover, Options: opts(1)}, want: nil},
		"success, len(opt 97) == 17":            {input: &dhcp4.Packet{Type: dhcp4.MsgDiscover, Options: opts(2)}, want: nil},
		"fail, missing opt 93":                  {input: &dhcp4.Packet{Type: dhcp4.MsgDiscover, Options: opts(3)}, want: errors.New("not a PXE boot request (missing option 93)")},
		"not discovery packet":                  {input: &dhcp4.Packet{Type: dhcp4.MsgAck}, want: fmt.Errorf("packet is %s, not %s or %s", dhcp4.MsgAck, dhcp4.MsgDiscover, dhcp4.MsgRequest)},
		"fail, len(opt 97) == 17, index 0 != 0": {input: &dhcp4.Packet{Type: dhcp4.MsgDiscover, Options: opts(4)}, want: errors.New("malformed client GUID (option 97), leading byte must be zero")},
		"fail, opt 97 wrong len":                {input: &dhcp4.Packet{Type: dhcp4.MsgDiscover, Options: opts(5)}, want: errors.New("malformed client GUID (option 97), wrong size")},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := isDiscoverPXEPacket(tc.input)
			if got != nil {
				if diff := cmp.Diff(got.Error(), tc.want.Error()); diff != "" {
					t.Fatal(diff)
				}
			} else {
				if diff := cmp.Diff(got, tc.want); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

func machineType(n int) machine {
	var mach machine
	switch n {
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
	default:
		mach.arch = Architecture(-1)
	}

	return mach
}

func opt93(n int) dhcp4.Options {
	opts := make(dhcp4.Options)
	switch n {
	case 0:
		opts[93] = []byte{0x0, 0x0}
	case 1:
		opts[93] = []byte{0x0, 0x1}
	case 2:
		opts[93] = []byte{0x0, 0x2}
	case 3:
		opts[93] = []byte{0x0, 0x3}
	case 4:
		opts[93] = []byte{0x0, 0x4}
	case 5:
		opts[93] = []byte{0x0, 0x5}
	case 6:
		opts[93] = []byte{0x0, 0x6}
	case 7:
		opts[93] = []byte{0x0, 0x7}
	case 8:
		opts[93] = []byte{0x0, 0x8}
	case 9:
		opts[93] = []byte{0x0, 0x9}
	case 10:
		opts[93] = []byte{0x0, 0x9}
		opts[77] = []byte("tinkerbell")
	case 31:
		opts[93] = []byte{0x0, 0x1F}
	}

	return opts
}

func TestProcessMachine(t *testing.T) {
	tests := map[string]struct {
		input       *dhcp4.Packet
		wantError   error
		wantMachine machine
	}{
		"success arch 0":        {input: &dhcp4.Packet{Options: opt93(0)}, wantError: nil, wantMachine: machineType(0)},
		"success arch 1":        {input: &dhcp4.Packet{Options: opt93(1)}, wantError: nil, wantMachine: machineType(1)},
		"success arch 2":        {input: &dhcp4.Packet{Options: opt93(2)}, wantError: nil, wantMachine: machineType(2)},
		"success arch 3":        {input: &dhcp4.Packet{Options: opt93(3)}, wantError: nil, wantMachine: machineType(3)},
		"success arch 4":        {input: &dhcp4.Packet{Options: opt93(4)}, wantError: nil, wantMachine: machineType(4)},
		"success arch 5":        {input: &dhcp4.Packet{Options: opt93(5)}, wantError: nil, wantMachine: machineType(5)},
		"success arch 6":        {input: &dhcp4.Packet{Options: opt93(6)}, wantError: nil, wantMachine: machineType(6)},
		"success arch 7":        {input: &dhcp4.Packet{Options: opt93(7)}, wantError: nil, wantMachine: machineType(7)},
		"success arch 8":        {input: &dhcp4.Packet{Options: opt93(8)}, wantError: nil, wantMachine: machineType(8)},
		"success arch 9":        {input: &dhcp4.Packet{Options: opt93(9)}, wantError: nil, wantMachine: machineType(9)},
		"fail, unknown arch 31": {input: &dhcp4.Packet{Options: opt93(31)}, wantError: fmt.Errorf("unsupported client firmware type '%d' (please file a bug!)", 31)},
		"fail, bad opt 93":      {input: &dhcp4.Packet{Options: opt93(12)}, wantError: fmt.Errorf("malformed DHCP option 93 (required for PXE): option not present in Options")},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m, err := processMachine(tc.input)
			if err != nil {
				if tc.wantError != nil {
					if diff := cmp.Diff(err.Error(), tc.wantError.Error()); diff != "" {
						t.Fatal(diff)
					}
				} else {
					t.Fatalf("expected nil error, got: %v", err)
				}
			} else {
				if diff := cmp.Diff(m, tc.wantMachine, cmp.AllowUnexported(machine{})); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

func TestCreateMSG(t *testing.T) {
	tests := map[string]struct {
		inputPkt  *dhcp4.Packet
		inputMach machine
		wantError error
		want      *dhcp4.Packet
	}{
		"success tftp": {
			inputPkt: &dhcp4.Packet{
				ServerAddr: net.IP{127, 0, 0, 1},
				Options: dhcp4.Options{
					97: {0x0, 0x0, 0x2, 0x0, 0x3, 0x0, 0x4, 0x0, 0x5, 0x0, 0x6, 0x0, 0x7, 0x0, 0x8, 0x0, 0x9},
				},
			},
			want: &dhcp4.Packet{
				Type:      dhcp4.MsgOffer,
				Broadcast: true,
				// ServerAddr:     net.IP{127, 0, 0, 1},
				// BootServerName: "127.0.0.1",
				// BootFilename:   "undionly.kpxe",
				Options: dhcp4.Options{
					43: {0x06, 0x01, 0x08, 0xff},
					// 54: {0x7f, 0x00, 0x00, 0x01},
					60: {0x50, 0x58, 0x45, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74},
					97: {0x0, 0x0, 0x2, 0x0, 0x3, 0x0, 0x4, 0x0, 0x5, 0x0, 0x6, 0x0, 0x7, 0x0, 0x8, 0x0, 0x9},
				},
			},
			inputMach: machineType(0),
		},
		"success http": {
			inputPkt: &dhcp4.Packet{
				ServerAddr: net.IP{127, 0, 0, 1},
				Options: dhcp4.Options{
					97: {0x0, 0x0, 0x2, 0x0, 0x3, 0x0, 0x4, 0x0, 0x5, 0x0, 0x6, 0x0, 0x7, 0x0, 0x8, 0x0, 0x9},
				},
			},
			want: &dhcp4.Packet{
				Type:      dhcp4.MsgOffer,
				Broadcast: true,
				// ServerAddr:   net.IP{127, 0, 0, 1},
				// BootFilename: "http://boot.netboot.xyz",
				Options: dhcp4.Options{
					43: {0x06, 0x01, 0x08, 0xff},
					// 54: {0x7f, 0x00, 0x00, 0x01},
					60: {0x50, 0x58, 0x45, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74},
					97: {0x0, 0x0, 0x2, 0x0, 0x3, 0x0, 0x4, 0x0, 0x5, 0x0, 0x6, 0x0, 0x7, 0x0, 0x8, 0x0, 0x9},
				},
			},
			inputMach: machineType(1),
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			pkt, err := withGenericHeaders(context.Background(), tc.inputPkt, tc.inputMach)
			if err != nil {
				if tc.wantError != nil {
					if diff := cmp.Diff(err.Error(), tc.wantError.Error()); diff != "" {
						t.Fatal(diff)
					}
				} else {
					t.Fatalf("expected nil error, got: %v", err)
				}
			} else {
				if diff := cmp.Diff(pkt, tc.want); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

func TestBootOpts(t *testing.T) {
	tests := map[string]struct {
		inputPkt  *dhcp4.Packet
		inputMach machine
		wantError error
		want      *dhcp4.Packet
	}{
		"success": {
			inputPkt: &dhcp4.Packet{
				Type:      dhcp4.MsgOffer,
				Broadcast: true,
				Options: dhcp4.Options{
					43: {0x06, 0x01, 0x08, 0xff},
					60: {0x50, 0x58, 0x45, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74},
					97: {0x0, 0x0, 0x2, 0x0, 0x3, 0x0, 0x4, 0x0, 0x5, 0x0, 0x6, 0x0, 0x7, 0x0, 0x8, 0x0, 0x9},
				},
			},
			want: &dhcp4.Packet{
				Type:           dhcp4.MsgOffer,
				Broadcast:      true,
				ServerAddr:     net.IP{127, 0, 0, 1},
				BootServerName: "127.0.0.1",
				BootFilename:   "undionly.kpxe",
				Options: dhcp4.Options{
					43: {0x06, 0x01, 0x08, 0xff},
					54: {0x7f, 0x00, 0x00, 0x01},
					60: {0x50, 0x58, 0x45, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74},
					97: {0x0, 0x0, 0x2, 0x0, 0x3, 0x0, 0x4, 0x0, 0x5, 0x0, 0x6, 0x0, 0x7, 0x0, 0x8, 0x0, 0x9},
				},
			},
			inputMach: machineType(0),
		},
	}
	loc := testLocator{ip: "127.0.0.1"}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			pkt, err := bootOpts(context.Background(), *tc.inputPkt, tc.inputMach, loc, "")
			if err != nil {
				if tc.wantError != nil {
					if diff := cmp.Diff(err.Error(), tc.wantError.Error()); diff != "" {
						t.Fatal(diff)
					}
				} else {
					t.Fatalf("expected nil error, got: %v", err)
				}
			} else {
				if diff := cmp.Diff(pkt, tc.want); diff != "" {
					t.Fatal(diff)
				}
			}
		})
	}
}

func TestArchString(t *testing.T) {
	tests := map[string]struct {
		input Architecture
		want  string
	}{
		"X86PC":           {input: X86PC, want: "Intel x86PC"},
		"NecPC98":         {input: NecPC98, want: "NEC/PC98"},
		"EfiItanium":      {input: EFIItanium, want: "EFI Itanium"},
		"DecAlpha":        {input: DecAlpha, want: "DEC Alpha"},
		"Arcx86":          {input: Arcx86, want: "Arc x86"},
		"IntelLeanClient": {input: IntelLeanClient, want: "Intel Lean Client"},
		"EfiIA32":         {input: EFIIA32, want: "EFI IA32"},
		"Efix8664":        {input: EFIx8664, want: "EFI x86-64"},
		"EfiXscale":       {input: EFIXscale, want: "EFI Xscale"},
		"EfiBC":           {input: EFIBC, want: "EFI BC"},
		"unknown":         {input: Architecture(20), want: "unknown architecture: 20"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(tc.input.String(), tc.want); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}
