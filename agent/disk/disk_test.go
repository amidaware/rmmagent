package disk_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/amidaware/rmmagent/agent/disk"
)

func TestGetDisks(t *testing.T) {
	testTable := []struct {
		name          string
		expected      []disk.Disk
		atLeast       int
		expectedError error
	}{
		{
			name:          "Get Disks",
			expected:      []disk.Disk{},
			atLeast:       1,
			expectedError: nil,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result, err := disk.GetDisks()
			if fmt.Sprintf("%T", result) != "[]disk.Disk" {
				t.Errorf("expected type %T, got type %T", tt.expected, result)
			}

			if !errors.Is(err, tt.expectedError) {
				t.Errorf("expected error (%v), got error(%v)", tt.expectedError, err)
			}

			if len(result) < 1 {
				t.Errorf("expected count at least %d, got %d", tt.atLeast, len(result))
			}
		})
	}
}
