package proxy

import (
	"fmt"
)

// Architecture describes a kind of CPU architecture.
type Architecture int

// UserClass is DHCP option 77 (https://www.rfc-editor.org/rfc/rfc3004.html).
type UserClass string

// clientType is from DHCP option 60. Normally on PXEClient or HTTPClient.
type clientType string

const (
	pxeClient  clientType = "PXEClient"
	httpClient clientType = "HTTPClient"
)

// known user-class types. must correspond to DHCP option 77 - User-Class
// https://www.rfc-editor.org/rfc/rfc3004.html
const (
	// If the client has had iPXE burned into its ROM (or is a VM
	// that uses iPXE as the PXE "ROM"), special handling is
	// needed because in this mode the client is using iPXE native
	// drivers and chainloading to a UNDI stack won't work.
	IPXE UserClass = "iPXE"
	// If the client identifies as "Tinkerbell", we've already
	// chainloaded this client to the full-featured copy of iPXE
	// we supply. We have to distinguish this case so we don't
	// loop on the chainload step.
	Tinkerbell UserClass = "Tinkerbell"
)

// known architecture types. must correspond to DHCP option 93 - Client System Architecture
// https://www.rfc-editor.org/errata_search.php?rfc=4578
const (
	X86PC           Architecture = iota // "Classic" x86 BIOS with PXE/UNDI support
	NecPC98                             // NEC/PC98
	EFIItanium                          // EFI Itanium
	DecAlpha                            // DEC Alpha
	Arcx86                              // Arc x86
	IntelLeanClient                     // Intel Lean Client
	EFIIA32                             // EFI IA32, 32-bit x86 processor running EFI
	EFIx8664                            // EFI x86-64, "Classic" x86 BIOS running iPXE (no UNDI support)
	EFIXscale                           // EFI Xscale
	EFIBC                               // EFI BC
	// https://github.com/tianocore/edk2/blob/ef5dcba975ee3b4c29b19ad0b23d371a2cd9d60a/MdePkg/Include/IndustryStandard/Dhcp.h#L258-L272
	// https://www.iana.org/assignments/dhcpv6-parameters/dhcpv6-parameters.xhtml#processor-architecture
	EFIARM                  // EFI ARM x86, uefi 32 for PXE
	EFIAARCH64              // EFI ARM x86_64, uefi 64 for PXE
	EFIx86Http     = 0x000f // EFI x86 HTTP, x86 uefi boot from http
	EFIx8664Http   = 0x0010 // EFI x86-64 HTTP, x86_64 uefi boot from http
	EFIARMHttp     = 0x0012 // EFI ARM x86 HTTP, Arm uefi 32 boot from http
	EFIAARCH64Http = 0x0013 // EFI ARM x86-64 HTTP, Arm uefi 64 boot from http
)

/*
https://www.rfc-editor.org/errata_search.php?rfc=4578
Type   Architecture Name
----   -----------------
  0    Intel x86PC
  1    NEC/PC98
  2    EFI Itanium
  3    DEC Alpha
  4    Arc x86
  5    Intel Lean Client
  6    EFI IA32
  7    EFI x86-64
  8    EFI Xscale
  9    EFI BC
*/

var (
	Defaults = map[Architecture]string{
		X86PC:           "undionly.kpxe",
		NecPC98:         "undionly.kpxe",
		EFIItanium:      "undionly.kpxe",
		DecAlpha:        "undionly.kpxe",
		Arcx86:          "undionly.kpxe",
		IntelLeanClient: "undionly.kpxe",
		EFIIA32:         "ipxe.efi",
		EFIx8664:        "ipxe.efi",
		EFIXscale:       "ipxe.efi",
		EFIBC:           "ipxe.efi",
		EFIARM:          "snp.efi",
		EFIAARCH64:      "snp.efi",
	}
	DefaultsHTTP = map[Architecture]string{
		EFIx86Http:     "http://%v/ipxe.efi",
		EFIx8664Http:   "http://%v/ipxe.efi",
		EFIARMHttp:     "http://%v/snp.efi",
		EFIAARCH64Http: "http://%v/snp.efi",
	}
)

func (a Architecture) String() string {
	switch a {
	case X86PC:
		return "Intel x86PC"
	case NecPC98:
		return "NEC/PC98"
	case EFIItanium:
		return "EFI Itanium"
	case DecAlpha:
		return "DEC Alpha"
	case Arcx86:
		return "Arc x86"
	case IntelLeanClient:
		return "Intel Lean Client"
	case EFIIA32:
		return "EFI IA32"
	case EFIx8664:
		return "EFI x86-64"
	case EFIXscale:
		return "EFI Xscale"
	case EFIBC:
		return "EFI BC"
	case EFIARM:
		return "EFI ARM x86"
	case EFIAARCH64:
		return "EFI ARM x86_64"
	case EFIx86Http:
		return "EFI x86 HTTP"
	case EFIx8664Http:
		return "EFI x86-64 HTTP"
	case EFIARMHttp:
		return "EFI ARM x86 HTTP"
	case EFIAARCH64Http:
		return "EFI ARM x86-64 HTTP"
	default:
		return fmt.Sprintf("unknown architecture: %d", a)
	}
}
