package patching_test

import (
	"errors"
	"testing"

	"github.com/amidaware/rmmagent/agent/patching"
)

func TestPatchMgmnt(t *testing.T) {
	testTable := []struct {
		name          string
		expectedError error
		status        bool
	}{
		{
			name:          "Enable Patch Mgmnt",
			expectedError: nil,
			status:        true,
		},
		{
			name:          "Disable Patch Mgmnt",
			expectedError: nil,
			status:        false,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			err := patching.PatchMgmnt(tt.status)
			if err != tt.expectedError {
				t.Errorf("expected error (%v), got error (%v)", tt.expectedError, err)
			}
		})
	}
}

func TestGetUpdates(t *testing.T) {
	testTable := []struct {
		name          string
		expectedError error
		atLeast       int
	}{
		{
			name:          "Get Updates",
			expectedError: nil,
			atLeast:       1,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result, err := patching.GetUpdates()
			if !errors.Is(tt.expectedError, err) {
				t.Errorf("expected (%v), got (%v)", tt.expectedError, err)
			}

			if len(result) < tt.atLeast {
				t.Errorf("expected at least %d, got %d", tt.atLeast, len(result))
			}
		})
	}
}
