package wmi_test

import (
	"reflect"
	"testing"

	"github.com/amidaware/rmmagent/agent/wmi"
)

func TestGetWMIInfo(t *testing.T) {
	testTable := []struct {
		name           string
		expected       map[string]interface{}
		atLeast        int
		expectedErrors []error
	}{
		{
			name:           "Get WMI Data",
			expected:       make(map[string]interface{}),
			atLeast:        1,
			expectedErrors: []error{},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result, errs := wmi.GetWMIInfo()
			if len(result) < tt.atLeast {
				t.Errorf("expected at least %d, got %d", tt.atLeast, len(result))
			}

			if !reflect.DeepEqual(errs, tt.expectedErrors) {
				t.Errorf("expected errors (%v), got (%v)", tt.expectedErrors, errs)
			}
		})
	}
}
