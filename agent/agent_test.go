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