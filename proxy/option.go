package proxy

import (
	"github.com/insomniacslk/dhcp/iana"
)

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

var ArchToBootFile = map[iana.Arch]string{
	iana.INTEL_X86PC:       "undionly.kpxe",
	iana.NEC_PC98:          "undionly.kpxe",
	iana.EFI_ITANIUM:       "undionly.kpxe",
	iana.DEC_ALPHA:         "undionly.kpxe",
	iana.ARC_X86:           "undionly.kpxe",
	iana.INTEL_LEAN_CLIENT: "undionly.kpxe",
	iana.EFI_IA32:          "ipxe.efi",
	iana.EFI_X86_64:        "ipxe.efi",
	iana.EFI_XSCALE:        "ipxe.efi",
	iana.EFI_BC:            "ipxe.efi",
	iana.EFI_ARM32:         "snp.efi",
	iana.EFI_ARM64:         "snp.efi",
	iana.EFI_X86_HTTP:      "ipxe.efi",
	iana.EFI_X86_64_HTTP:   "ipxe.efi",
	iana.EFI_ARM32_HTTP:    "snp.efi",
	iana.EFI_ARM64_HTTP:    "snp.efi",
}
