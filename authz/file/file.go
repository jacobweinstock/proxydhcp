// Package file implements a file-based authorization policy for PXE enabled clients.
package file

import (
	"context"
	"net"

	"github.com/tinkerbell/tink/protos/hardware"
)

// File holds a slice of hardware records.
type File struct {
	DB []*hardware.Hardware
}

// Allow checks if a mac address exists in the DB and returns it's allow_pxe field or false.
func (f File) Allow(_ context.Context, mac net.HardwareAddr) bool {
	for _, v := range f.DB {
		for _, hip := range v.Network.Interfaces {
			if hw, err := net.ParseMAC(hip.Dhcp.Mac); err == nil {
				if hw.String() == mac.String() {
					return hip.Netboot.AllowPxe
				}
			}
		}
	}
	return false
}
