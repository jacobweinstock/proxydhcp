package proxy

// https://github.com/danderson/netboot/blob/bdaec9d82638460bf166fb98bdc6d97331d7bd80/dhcp4/testdata/dhcp.parsed
/*
func TestServe(t *testing.T) {
	tests := map[string]struct {
		input string
		want  error
	}{
		"context canceled": {input: "127.0.0.1:60656", want: context.Canceled},
	}
	// tl := testLocator{ip: "127.0.0.1"}

	logger := logr.Discard()

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
				Serve(ctx, logger, conn, "0.0.0.0", "0.0.0.0", "0.0.0.0", "auto.ipxe", "tinkerbell")
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

	mac, err := net.ParseMAC("B8:27:EB:ef:45:f7")
	if err != nil {
		fmt.Println("2", err)
		return
	}
	opts := make(dhcp4.Options)
	var opt93 dhcp4.Option = 93
	opts[opt93] = []byte{0x0, 0x0}
	opts[94] = []byte{0x0, 0x0}
	opts[60] = []byte("PXEClient")
	opts[97] = []byte{}
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

		//pc, err := reuseport.Dial("udp4", "192.168.2.225:35689", "")
		//if err != nil {
		//	fmt.Println("45 err", err)
		//	return
		//}
		//defer pc.Close()

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
	opts := dhcp4.Options{
		60: []byte("PXEClient:Arch:xxxxx:UNDI:yyyzzz"),
		93: {0x0, 0x0},
		94: {},
	}
	switch num {
	case 1:
	case 2:
		opts[97] = []byte{0x0, 0x0, 0x2, 0x0, 0x3, 0x0, 0x4, 0x0, 0x5, 0x0, 0x6, 0x0, 0x7, 0x0, 0x8, 0x0, 0x9}
	case 3:
		opts[93] = nil
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
		"not discovery packet":                  {input: &dhcp4.Packet{Type: dhcp4.MsgAck}, want: fmt.Errorf("DHCP message type is DHCPACK, must be DHCPDISCOVER")},
		"fail, len(opt 97) == 17, index 0 != 0": {input: &dhcp4.Packet{Type: dhcp4.MsgDiscover, Options: opts(4)}, want: errors.New("malformed client GUID (option 97), leading byte must be zero")},
		"fail, opt 97 wrong len":                {input: &dhcp4.Packet{Type: dhcp4.MsgDiscover, Options: opts(5)}, want: errors.New("malformed client GUID (option 97), wrong size")},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := isDiscoverPXEPacket(tc.input)
			if got != nil {
				if tc.want != nil {
					if diff := cmp.Diff(got.Error(), tc.want.Error()); diff != "" {
						t.Fatal(diff)
					}
				} else {
					t.Fatalf("expected a nil error, got: %q", got)
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
		mach.arch = Architecture(-1)
	}
	mac, _ := net.ParseMAC("00:00:5e:00:53:01")
	mach.mac = mac

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
		opts[93] = []byte{0x0, 10}
		// opts[77] = []byte("tinkerbell")
	case 11:
		opts[93] = []byte{0x0, 11}
	case 15:
		opts[93] = []byte{0x0, 15}
	case 16:
		opts[93] = []byte{0x0, 16}
	case 18:
		opts[93] = []byte{0x0, 18}
	case 19:
		opts[93] = []byte{0x0, 19}
	case 31:
		opts[93] = []byte{0x0, 0x1F}
	}

	return opts
}

func TestProcessMachine(t *testing.T) {
	mac, _ := net.ParseMAC("00:00:5e:00:53:01")
	tests := map[string]struct {
		input       *dhcp4.Packet
		wantError   error
		wantMachine machine
	}{
		"success arch 0":        {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(0)}, wantError: nil, wantMachine: machineType(0)},
		"success arch 1":        {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(1)}, wantError: nil, wantMachine: machineType(1)},
		"success arch 2":        {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(2)}, wantError: nil, wantMachine: machineType(2)},
		"success arch 3":        {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(3)}, wantError: nil, wantMachine: machineType(3)},
		"success arch 4":        {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(4)}, wantError: nil, wantMachine: machineType(4)},
		"success arch 5":        {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(5)}, wantError: nil, wantMachine: machineType(5)},
		"success arch 6":        {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(6)}, wantError: nil, wantMachine: machineType(6)},
		"success arch 7":        {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(7)}, wantError: nil, wantMachine: machineType(7)},
		"success arch 8":        {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(8)}, wantError: nil, wantMachine: machineType(8)},
		"success arch 9":        {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(9)}, wantError: nil, wantMachine: machineType(9)},
		"success arch 10":       {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(10)}, wantError: nil, wantMachine: machineType(10)},
		"success arch 11":       {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(11)}, wantError: nil, wantMachine: machineType(11)},
		"success arch 15":       {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(15)}, wantError: nil, wantMachine: machineType(15)},
		"success arch 16":       {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(16)}, wantError: nil, wantMachine: machineType(16)},
		"success arch 18":       {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(18)}, wantError: nil, wantMachine: machineType(18)},
		"success arch 19":       {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(19)}, wantError: nil, wantMachine: machineType(19)},
		"fail, unknown arch 31": {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(31)}, wantError: fmt.Errorf("unsupported client firmware type '%d' for %q (please file a bug!)", 31, mac)},
		"fail, bad opt 93":      {input: &dhcp4.Packet{HardwareAddr: mac, Options: opt93(12)}, wantError: fmt.Errorf("malformed DHCP option 93 (required for PXE): option not present in Options")},
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
*/

/*
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
*/

/*
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
*/
