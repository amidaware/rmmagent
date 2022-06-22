package choco_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/amidaware/rmmagent/agent/choco"
)

func TestInstallChoco(t *testing.T) {
	testTable := []struct {
		name          string
		expectedError error
	}{
		{
			name:          "Install Choco",
			expectedError: nil,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			err := choco.InstallChoco()
			if !errors.Is(tt.expectedError, err) {
				t.Errorf("expected error (%v), got (%v)", tt.expectedError, err)
			}
		})
	}
}

func TestInstallWithChoco(t *testing.T) {
	testTable := []struct {
		name           string
		software       string
		expectedString string
		expectedError  error
	}{
		{
			name:           "Install With Choco",
			software:       "adobereader",
			expectedString: "The install of adobereader was successful",
			expectedError:  nil,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result, err := choco.InstallWithChoco(tt.software)
			if !errors.Is(tt.expectedError, err) {
				t.Errorf("expected (%v), got (%v)", tt.expectedError, err)
			}

			if !strings.Contains(result, tt.expectedString) {
				t.Errorf("expected %s, got %s", tt.expectedString, result)
			}
		})
	}
}
