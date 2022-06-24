package agent

import (
	"errors"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

var (
	version = "2.0.4"
	lg     = logrus.New()
	a = New(lg, version)
)

func TestGetDisks(t *testing.T) {
	disks := a.GetDisks()
	if(len(disks) < 1) {
		t.Errorf("Could not get disks")
	}

	t.Logf("Result: %v", disks)
}

func TestAgentId(t *testing.T) {
	if a.AgentID == "" {
		t.Error("AgentID not set")
	} else {
		t.Logf("AgentID: %s", a.AgentID)
	}
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

func TestRunScript(t *testing.T){
	testTable := []struct {
		name string
		code string
		shell string
		args []string
		timeout int
		expectedStdout string
		expectedStderr string
		expectedExitcode int
		expectedError error
	}{
		{
			name: "Run Script",
			code: "#!/bin/sh\necho 'test'",
			shell: "/bin/sh",
			args: []string{},
			timeout: 30,
			expectedStdout: "test\n",
			expectedStderr: "",
			expectedExitcode: 0,
			expectedError: nil,
		},
		{
			name: "Run Bad Script No Hash Bang",
			code: "echo 'test'",
			shell: "/bin/sh",
			args: []string{},
			timeout: 30,
			expectedStdout: "",
			expectedStderr: "exec format error",
			expectedExitcode: -1,
			expectedError: nil,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitcode, err := a.RunScript(tt.code, tt.shell, tt.args, tt.timeout)
			if tt.expectedStdout != stdout {
				t.Errorf("expected %s, got %s", tt.expectedStdout, stdout)
			}

			if !strings.Contains(stderr, tt.expectedStderr) {
				t.Errorf("expected stderr to contain %s, got %s", tt.expectedStderr, stderr)
			}

			if tt.expectedExitcode != exitcode {
				t.Errorf("expected exit %d, got exit %d", tt.expectedExitcode, exitcode)
			}

			if !errors.Is(tt.expectedError, err) {
				t.Errorf("expected (%v), got (%v)", tt.expectedError, err)
			}
		})
	}
}