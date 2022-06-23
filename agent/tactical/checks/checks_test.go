package checks_test

import (
	"errors"
	"testing"

	"github.com/amidaware/rmmagent/agent/tactical/checks"
	"github.com/amidaware/rmmagent/agent/tactical/config"
)

func TestGetCheckInterval(t *testing.T) {
	config := config.NewAgentConfig()
	testTable := []struct {
		name string
		interval int
		expectedError error
	}{
		{
			name: "Get Check Interval",
			interval: 1,
			expectedError: nil,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result, err := checks.GetCheckInterval(config.AgentID)
			if result < tt.interval {
				t.Errorf("expected greater interval than %d, got %d", tt.interval, result)
			}

			if !errors.Is(tt.expectedError, err) {
				t.Errorf("expected (%v), got (%v)", tt.expectedError, err)
			}
		})
	}
}

func TestCheckRunner(t *testing.T) {
	config := config.NewAgentConfig()
	testTable := []struct {
		name string
		expectedError error
	}{
		{
			name: "Check Runner",
			expectedError: nil,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			err := checks.CheckRunner(config.AgentID)
			if !errors.Is(tt.expectedError, err) {
				t.Errorf("expected (%v), got (%v)", tt.expectedError, err)
			}
		})
	}
}

func TestRunChecks(t *testing.T) {
	config := config.NewAgentConfig()
	testTable := []struct {
		name string
		expectedError error
		force bool
		agentId string
	}{
		{
			name: "Run Checks",
			expectedError: nil,
			force: false,
			agentId: config.AgentID,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T){
			err := checks.RunChecks(tt.agentId, tt.force)
			if !errors.Is(tt.expectedError, err) {
				t.Errorf("expected (%v), got (%v)", tt.expectedError, err)
			}
		})
	}
}