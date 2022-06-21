package utils_test

import (
	"testing"

	"github.com/amidaware/rmmagent/agent/utils"
)

func TestByteCountSI(t *testing.T) {
	testTable := []struct {
		name     string
		expected string
		bytes    uint64
	}{
		{
			name:     "Bytes to Kilobytes",
			expected: "1.0 kB",
			bytes:    1024,
		},
		{
			name:     "Bytes to Megabytes",
			expected: "1.0 MB",
			bytes:    1048576,
		},
		{
			name:     "Bytes to Gigabytes",
			expected: "1.0 GB",
			bytes:    1073741824,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.ByteCountSI(tt.bytes)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
