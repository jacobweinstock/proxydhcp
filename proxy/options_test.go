package proxy

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

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
		"EFIARM":          {input: EFIARM, want: "EFI ARM x86"},
		"EFIAARCH64":      {input: EFIAARCH64, want: "EFI ARM x86_64"},
		"EFIx86Http":      {input: EFIx86Http, want: "EFI x86 HTTP"},
		"EFIx8664Http":    {input: EFIx8664Http, want: "EFI x86-64 HTTP"},
		"EFIARMHttp":      {input: EFIARMHttp, want: "EFI ARM x86 HTTP"},
		"EFIAARCH64Http":  {input: EFIAARCH64Http, want: "EFI ARM x86-64 HTTP"},
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
