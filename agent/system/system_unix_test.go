//go:build darwin
// +build darwin

package system_test

import (
	"testing"

	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/utils"
)

func TestNewCMDOpts(t *testing.T) {
	opts := system.NewCMDOpts()
	if opts.Shell != "/bin/bash" {
		t.Fatalf("Expected /bin/bash, got %s", opts.Shell)
	}
}

func TestSystemRebootRequired(t *testing.T) {
	required, err := system.SystemRebootRequired()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("System Reboot Required %t", required)
}

func TestShowStatus(t *testing.T) {
	output := utils.CaptureOutput(func() {
		system.ShowStatus("1.0.0")
	})

	if output != "1.0.0\n" {
		t.Fatalf("Expected 1.0.0, got %s", output)
	}
}

func TestLoggedOnUser(t *testing.T) {
	user := system.LoggedOnUser()
	if user == "" {
		t.Fatalf("Expected a user, got empty")
	}

	t.Logf("Logged on user: %s", user)
}

func TestOsString(t *testing.T) {
	osString := system.OsString()
	if osString == "error getting host info" {
		t.Fatalf("Unable to get OS string")
	}

	t.Logf("OS String: %s", osString)
}