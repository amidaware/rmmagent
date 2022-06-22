package software_test

import (
	"errors"
	"testing"

	"github.com/amidaware/rmmagent/agent/software"
)

func TestGetInstalledSoftware(t *testing.T) {
	testTable := []struct {
		name          string
		expected      []software.Software
		atLeast       int
		expectedError error
	}{
		{
			name:          "Get Installed Software",
			expected:      []software.Software{},
			atLeast:       1,
			expectedError: nil,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result, err := software.GetInstalledSoftware()
			t.Logf("result: (%v)", result)
			if len(result) < tt.atLeast {
				t.Errorf("expected at least %d, got %d", tt.atLeast, len(result))
			}

			if !errors.Is(tt.expectedError, err) {
				t.Errorf("expected (%v), got (%v)", tt.expectedError, err)
			}
		})
	}
}
