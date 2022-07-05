//go:build darwin
// +build darwin

package system_test

import (
	"testing"

	"github.com/amidaware/rmmagent/agent/system"
)

func TestRunScript(t *testing.T) {
	_, stderr, exitcode, err := system.RunScript("#!/bin/sh\nuname -av", "/bin/bash", nil, 30)
	if err != nil {
		t.Fatal(err)
	}

	if stderr != "" {
		t.Fatal(stderr)
	}

	if exitcode != 0 {
		t.Fatalf("Error: Exit Code %d", exitcode)
	}
}
