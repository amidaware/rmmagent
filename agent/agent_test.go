package agent

import (
	"testing"
	"github.com/sirupsen/logrus"
)

var (
	version = "2.0.4"
	lg     = logrus.New()
)

func TestAgentId(t *testing.T) {
	a := New(lg, version)
	if a.AgentID == "" {
		t.Error("AgentID not set")
	} else {
		t.Logf("AgentID: %s", a.AgentID)
	}
}

func TestSystemRebootRequired(t *testing.T) {
	a := New(lg, version)
	result, err := a.SystemRebootRequired()
	if err != nil {
		t.Error(err)
	}

	t.Logf("Result: %t", result)
}

func TestLoggedOnUser(t *testing.T) {
	a := New(lg, version)
	result := a.LoggedOnUser()
	if result == "" {
		t.Errorf("Could not get logged on user.")
	}

	t.Logf("Result: %s", result)
}

