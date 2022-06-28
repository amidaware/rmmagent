package agent

import (
	"testing"

	"github.com/sirupsen/logrus"
)

var (
	version = "2.0.4"
	lg      = logrus.New()
	a       = New(lg, version)
)

func TestGetDisks(t *testing.T) {
	disks := a.GetDisks()
	if len(disks) < 1 {
		t.Errorf("Could not get disks")
	}

	t.Logf("Result: %v", disks)
}

func TestSystemRebootRequired(t *testing.T) {
	result, err := a.SystemRebootRequired()
	if err != nil {
		t.Error(err)
	}

	t.Logf("Result: %t", result)
}

func TestLoggedOnUser(t *testing.T) {
	result := a.LoggedOnUser()
	if result == "" {
		t.Errorf("Could not get logged on user.")
	}

	t.Logf("Result: %s", result)
}
