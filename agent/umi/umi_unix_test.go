//go:build !windows
// +build !windows

package umi_test

import (
	"reflect"
	"testing"

	"github.com/amidaware/rmmagent/agent/umi"
)

func TestGetInfo(t *testing.T) {
	testTable := []struct {
		name string
		expected map[string]interface{}
		atLeast int
		expectedErrors []error
	}{
		{
			name: "Get info",
			expected: make(map[string]interface{}),
			atLeast: 1,
			expectedErrors: []error{},
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result, errs := umi.GetInfo()
			if len(result) < tt.atLeast {
				t.Errorf("expected at least %d, got %d", tt.atLeast, len(result))
			}

			if !reflect.DeepEqual(tt.expectedErrors, errs) {
				t.Errorf("expected (%v), got (%v)", tt.expectedErrors, errs)
			}
		})
	}
}
