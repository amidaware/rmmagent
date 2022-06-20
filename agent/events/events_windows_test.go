package events_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/amidaware/rmmagent/agent/events"
)

func TestGetEventLog(t *testing.T) {
	testTable := []struct {
		name          string
		expected      []events.EventLogMsg
		atLeast       int
		expectedError error
		logname       string
		search        int
	}{
		{
			name:          "Get EventLog",
			expected:      []events.EventLogMsg{},
			atLeast:       1,
			expectedError: nil,
			logname:       "Application",
			search:        1,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			result, err := events.GetEventLog(tt.logname, tt.search)
			if fmt.Sprintf("%T", result) != "[]events.EventLogMsg" {
				t.Errorf("expected type %T, got type %T", []events.EventLogMsg{}, result)
			}

			if !errors.Is(err, tt.expectedError) {
				t.Errorf("expected error (%v), got error (%v)", tt.expectedError, err)
			}

			if len(result) < 1 {
				t.Errorf("expected count at least %d, got %d", tt.atLeast, len(result))
			}
		})
	}
}
