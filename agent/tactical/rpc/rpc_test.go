package rpc_test

import (
	"errors"
	"testing"

	"github.com/amidaware/rmmagent/agent/tactical/rpc"
)

func TestRunRPC(t *testing.T) {
	testTable := []struct {
		name string
		expectedError error
		version string
	}{
		{
			name: "Run RPC",
			expectedError: nil, 
			version: "development",
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			err := rpc.RunRPC(tt.version)
			if !errors.Is(tt.expectedError, err) {
				t.Errorf("expected (%v), got (%v)", tt.expectedError, err)
			}
		})
	}
}