package file

import (
	"context"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tinkerbell/tink/protos/hardware"
)

func TestAllow(t *testing.T) {
	mactests := map[string]struct {
		mac     string
		allowed bool
	}{
		"ip allowed":                    {"0a:00:27:00:00:00", true},
		"ip not allowed":                {"0a:00:27:00:00:00", false},
		"ip not allowed - no mac found": {"", false},
	}
	for name, tc := range mactests {
		t.Run(name, func(t *testing.T) {
			record := File{DB: []*hardware.Hardware{{
				Network: &hardware.Hardware_Network{
					Interfaces: []*hardware.Hardware_Network_Interface{
						{
							Dhcp: &hardware.Hardware_DHCP{
								Mac: tc.mac,
							},
							Netboot: &hardware.Hardware_Netboot{},
						},
					},
				},
			}}}
			record.DB[0].Network.Interfaces[0].Netboot.AllowPxe = tc.allowed
			hw, _ := net.ParseMAC(tc.mac)
			got := record.Allow(context.TODO(), hw)
			if cmp.Diff(got, tc.allowed) != "" {
				t.Fatalf("got %v, want %v", got, tc.allowed)
			}
		})
	}
}
