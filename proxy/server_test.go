package proxy

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGetInterfaceByIP(t *testing.T) {
	tests := []struct {
		name   string
		ip     string
		wantIF string
	}{
		{
			name:   "success",
			ip:     "127.0.0.1",
			wantIF: "lo0",
		},
		{
			name:   "not found",
			ip:     "1.1.1.1",
			wantIF: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(getInterfaceByIP(tt.ip), tt.wantIF); diff != "" {
				t.Fatalf(diff)
			}
		})
	}
}
