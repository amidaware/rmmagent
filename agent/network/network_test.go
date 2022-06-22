package network_test

import (
	"testing"

	"github.com/amidaware/rmmagent/agent/network"
)

func TestPublicIP(t *testing.T) {
	testTable := []struct {
		name     string
		expected string
		proxy    string
	}{
		{
			name:     "Get Public IP",
			expected: network.PublicIP(""),
			proxy:    "",
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result := network.PublicIP(tt.proxy)
			t.Logf("result: (%v)", result)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
