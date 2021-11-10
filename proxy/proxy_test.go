package proxy

import (
	"errors"
	"net"
	"testing"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
)

func TestValidatePXE(t *testing.T) {
	tests := []struct {
		name    string
		mods    []dhcpv4.Modifier
		wantErr error
	}{
		{
			name: "failure unknown DHCP message type",
			mods: []dhcpv4.Modifier{
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeInform))
				},
			},
			wantErr: ErrInvalidMsgType{Invalid: dhcpv4.MessageTypeInform},
		},
		{
			name: "failure DHCP option 60 not set",
			mods: []dhcpv4.Modifier{
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover))
				},
			},
			wantErr: ErrOpt60Missing,
		},
		{
			name: "failure invalid option 60",
			mods: []dhcpv4.Modifier{
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClassIdentifier, []byte("notValid")))
				},
			},
			wantErr: ErrInvalidOption60{Opt60: string("notValid")},
		},
		{
			name: "failure Option 93 missing",
			mods: []dhcpv4.Modifier{
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClassIdentifier, []byte("PXEClient:Arch:xxxxx:UNDI:yyyzzz")))
				},
			},
			wantErr: ErrOpt93Missing,
		},
		{
			name: "failure Option 94 missing",
			mods: []dhcpv4.Modifier{
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClassIdentifier, []byte("PXEClient:Arch:xxxxx:UNDI:yyyzzz")))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClientSystemArchitectureType, iana.Archs{iana.ARC_X86}.ToBytes()))
				},
			},
			wantErr: ErrOpt94Missing,
		},
		{
			name: "failure Option 97 invalid",
			mods: []dhcpv4.Modifier{
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClassIdentifier, []byte("PXEClient:Arch:xxxxx:UNDI:yyyzzz")))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClientSystemArchitectureType, iana.Archs{iana.ARC_X86}.ToBytes()))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClientNetworkInterfaceIdentifier, []byte{1, 2, 1}))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClientMachineIdentifier, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3, 4, 5, 6, 7, 8}))
				},
			},
			wantErr: ErrOpt97LeadingByteError,
		},
		{
			name: "failure Option 97 wrong size",
			mods: []dhcpv4.Modifier{
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClassIdentifier, []byte("PXEClient:Arch:xxxxx:UNDI:yyyzzz")))
					d.UpdateOption(dhcpv4.OptClientArch(iana.EFI_X86_64))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClientNetworkInterfaceIdentifier, []byte{1, 2, 1}))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClientMachineIdentifier, []byte{1}))
				},
			},
			wantErr: ErrOpt97WrongSize,
		},
		{
			name: "success",
			mods: []dhcpv4.Modifier{
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClassIdentifier, []byte("PXEClient:Arch:xxxxx:UNDI:yyyzzz")))
					d.UpdateOption(dhcpv4.OptClientArch(iana.EFI_X86_64))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClientNetworkInterfaceIdentifier, []byte{1, 2, 1}))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClientMachineIdentifier, []byte{0, 2, 3, 4, 5, 6, 7, 8, 9, 1, 2, 3, 4, 5, 6, 7, 8}))
				},
			},
			wantErr: nil,
		},
		{
			name: "success len(opt97) == 0",
			mods: []dhcpv4.Modifier{
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeRequest))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClassIdentifier, []byte("PXEClient:Arch:xxxxx:UNDI:yyyzzz")))
					d.UpdateOption(dhcpv4.OptClientArch(iana.EFI_X86_64))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClientNetworkInterfaceIdentifier, []byte{1, 2, 1}))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionClientMachineIdentifier, []byte{}))
				},
			},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := dhcpv4.New(tt.mods...)
			if err != nil {
				t.Fatal(err)
			}
			r := replyPacket{
				DHCPv4: &dhcpv4.DHCPv4{},
				log:    logr.Discard(),
			}
			if err := r.validatePXE(m); !errors.Is(err, tt.wantErr) {
				t.Errorf("validateDiscover() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestProcessMachine(t *testing.T) {
	tests := []struct {
		name     string
		mods     []dhcpv4.Modifier
		mac      net.HardwareAddr
		wantMach machine
		wantErr  error
	}{
		{
			name: "failure unknown architecture",
			mods: []dhcpv4.Modifier{
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover))
				},
			},
			mac:     net.HardwareAddr{0x00, 0x01, 0x02, 0x03, 0x04, 0x05},
			wantErr: ErrUnknownArch,
		},
		{
			name: "success",
			mods: []dhcpv4.Modifier{
				func(d *dhcpv4.DHCPv4) {
					d.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeDiscover))
					d.UpdateOption(dhcpv4.OptClientArch(iana.EFI_X86_64))
					d.UpdateOption(dhcpv4.OptGeneric(dhcpv4.OptionUserClassInformation, []byte("Tinkerbell")))
				},
			},
			mac: net.HardwareAddr{0x00, 0x01, 0x02, 0x03, 0x04, 0x05},
			wantMach: machine{
				mac:    net.HardwareAddr{0x00, 0x01, 0x02, 0x03, 0x04, 0x05},
				arch:   iana.EFI_X86_64,
				uClass: "Tinkerbell",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := dhcpv4.New(tt.mods...)
			if err != nil {
				t.Fatal(err)
			}
			m.ClientHWAddr = tt.wantMach.mac
			mach, err := processMachine(m)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("processMachine() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr == nil {
				if diff := cmp.Diff(mach, tt.wantMach, cmp.AllowUnexported(machine{})); diff != "" {
					t.Fatalf(diff)
				}
			}
		})
	}
}
