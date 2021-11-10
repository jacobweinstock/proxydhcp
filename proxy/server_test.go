package proxy

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetInterfaceByIP(t *testing.T) {
	tests := []struct {
		name   string
		ip     string
		wantIF []string
	}{
		{
			name:   "success",
			ip:     "127.0.0.1",
			wantIF: []string{"lo0", "lo"},
		},
		{
			name:   "not found",
			ip:     "1.1.1.1",
			wantIF: []string{""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var diffs []string
			for _, want := range tt.wantIF {
				diff := cmp.Diff(getInterfaceByIP(tt.ip), want)
				if diff != "" {
					diffs = append(diffs, diff)
				}
			}
			if len(diffs) == len(tt.wantIF) {
				t.Fatalf("%v", diffs)
			}
		})
	}
}
