package services_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/amidaware/rmmagent/agent/services"
	"golang.org/x/sys/windows"
)

func TestGetServices(t *testing.T) {
	testTable := []struct {
		name          string
		expected      []services.Service
		atLeast       int
		expectedError error
	}{
		{
			name:          "Get Services",
			expected:      []services.Service{},
			atLeast:       1,
			expectedError: nil,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result, errs, err := services.GetServices()
			if fmt.Sprintf("%T", result) != "[]services.Service" {
				t.Errorf("expected type %T, got type %T", tt.expected, result)
			}

			if len(errs) > 0 {
				t.Logf("Continue errors occured %v", errs)
			}

			if err != nil {
				t.Errorf("expected error (%v), got error(%v)", tt.expectedError, err)
			}

			if len(result) < tt.atLeast {
				t.Errorf("expect at least %d, got %d", tt.atLeast, len(result))
			}
		})
	}
}

func TestGetServiceStatus(t *testing.T) {
	testTable := []struct {
		name          string
		expected      string
		expectedError error
	}{
		{
			name:          "CryptSvc",
			expected:      "running",
			expectedError: nil,
		},
		{
			name:          "NonExistentService",
			expected:      "n/a",
			expectedError: windows.ERROR_SERVICE_DOES_NOT_EXIST,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result, err := services.GetServiceStatus(tt.name)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}

			if !errors.Is(err, tt.expectedError) {
				t.Errorf("expected (%v), got (%v)", tt.expectedError, err)
			}
		})
	}
}
