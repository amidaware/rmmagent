package wua_test

import (
	"errors"
	"testing"

	wua "github.com/amidaware/rmmagent/agent/patching/wua"
)

func TestWUAUpdates(t *testing.T) {
	testTable := []struct {
		name          string
		expected      []wua.WUAPackage
		atLeast       int
		expectedError error
		query         string
	}{
		{
			name:          "Get WUA Updates",
			expected:      []wua.WUAPackage{},
			atLeast:       1,
			expectedError: nil,
			query:         "IsInstalled=1 or IsInstalled=0 and Type='Software' and IsHidden=0",
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result, err := wua.WUAUpdates(tt.query)
			if len(result) < tt.atLeast {
				t.Errorf("expected at least %d, got %d", tt.atLeast, len(result))
			}

			if !errors.Is(tt.expectedError, err) {
				t.Errorf("expected (%v), got (%v)", tt.expectedError, err)
			}
		})
	}
}
