//go:build !windows
// +build !windows

package umi_test

import (
	"testing"

	"github.com/amidaware/rmmagent/agent/umi"
)

func TestGetInfo(t *testing.T) {
	testTable := []struct {
		name string
		expected map[string]interface{}
		atLeast int
	}{
		{
			name: "Get info",
			expected: make(map[string]interface{}),
			atLeast: 1,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result, _ := umi.GetInfo()
			if len(result) < tt.atLeast {
				t.Errorf("expected at least %d, got %d", tt.atLeast, len(result))
			}
		})
	}
}
