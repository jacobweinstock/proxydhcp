package proxy

import "fmt"

// Architecture describes a kind of CPU architecture.
type Architecture int

// UserClass is DHCP option 77 (https://www.rfc-editor.org/rfc/rfc3004.html).
type UserClass string

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
	Tinkerbell UserClass = "tinkerbell"
)

// known architecture types. must correspond to DHCP option 93 - Client System Architecture
// https://www.rfc-editor.org/errata_search.php?rfc=4578
const (
	X86PC           Architecture = iota // "Classic" x86 BIOS with PXE/UNDI support
	NecPC98                             // NEC/PC98
	EfiItanium                          // EFI Itanium
	DecAlpha                            // DEC Alpha
	Arcx86                              // Arc x86
	IntelLeanClient                     // Intel Lean Client
	EfiIA32                             // EFI IA32, 32-bit x86 processor running EFI
	Efix8664                            // EFI x86-64, "Classic" x86 BIOS running iPXE (no UNDI support)
	EfiXscale                           // EFI Xscale
	EfiBC                               // EFI BC
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

func (a Architecture) String() string {
	switch a {
	case X86PC:
		return "Intel x86PC"
	case NecPC98:
		return "NEC/PC98"
	case EfiItanium:
		return "EFI Itanium"
	case DecAlpha:
		return "DEC Alpha"
	case Arcx86:
		return "Arc x86"
	case IntelLeanClient:
		return "Intel Lean Client"
	case EfiIA32:
		return "EFI IA32"
	case Efix8664:
		return "EFI x86-64"
	case EfiXscale:
		return "EFI Xscale"
	case EfiBC:
		return "EFI BC"
	default:
		return fmt.Sprintf("unknown architecture: %d", a)
	}
}
