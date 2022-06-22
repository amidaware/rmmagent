//go:build !windows
// +build !windows

package system

import (
	"testing"
	"github.com/amidaware/rmmagent/agent/utils"
)

func TestNewCMDOpts(t *testing.T) {
	opts := NewCMDOpts()
	if opts.Shell != "/bin/bash" {
		t.Fatalf("Expected /bin/bash, got %s", opts.Shell)
	}
}

func TestSystemRebootRequired(t *testing.T) {
	required, err := SystemRebootRequired()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("System Reboot Required %t", required)
}

func TestShowStatus(t *testing.T) {
	output := utils.CaptureOutput(func() {
		ShowStatus("1.0.0")
	});

	if output != "1.0.0\n" {
		t.Fatalf("Expected 1.0.0, got %s", output)
	}
}

func TestLoggedOnUser(t *testing.T) {
	user := LoggedOnUser()
	if user == "" {
		t.Fatalf("Expected a user, got empty")
	}

	t.Logf("Logged on user: %s", user)
}

func TestOsString(t *testing.T) {
	osString := OsString()
	if osString == "error getting host info" {
		t.Fatalf("Unable to get OS string")
	}

	t.Logf("OS String: %s", osString)
}

func TestRunScript(t *testing.T) {
	stdout, stderr, exitcode, err := RunScript("#!/bin/sh\ncat /etc/os-release", "/bin/sh", nil, 30)
	if err != nil {
		t.Fatal(err)
	}

	if stderr != "" {
		t.Fatal(stderr)
	}

	if exitcode != 0 {
		t.Fatalf("Error: Exit Code %d", exitcode)
	}

	t.Logf("Result: %s", stdout)
}