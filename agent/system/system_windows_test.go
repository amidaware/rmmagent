package system_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/amidaware/rmmagent/agent/system"
)

func TestRunScript(t *testing.T) {
	testTable := []struct {
		name             string
		code             string
		shell            string
		args             []string
		timeout          int
		expectedStdout   string
		expectedStderr   string
		expectedExitCode int
		expectedError    error
	}{
		{
			name:             "Run Script",
			code:             "Test-Path -Path C:\\Windows",
			shell:            "powershell",
			args:             []string{},
			timeout:          30,
			expectedStdout:   "True\r\n",
			expectedStderr:   "",
			expectedExitCode: 0,
			expectedError:    nil,
		},
		{
			name:             "Run Error Script",
			code:             "Get-ThisError",
			shell:            "powershell",
			args:             []string{},
			timeout:          30,
			expectedStdout:   "",
			expectedStderr:   "The term 'Get-ThisError' is not recognized as the name of a cmdlet",
			expectedExitCode: 0,
			expectedError:    nil,
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, exitcode, err := system.RunScript(tt.code, tt.shell, tt.args, tt.timeout)
			if stdout != tt.expectedStdout {
				t.Errorf("expected stdout %s, got %s", tt.expectedStdout, stdout)
			}

			if !strings.Contains(stderr, tt.expectedStderr) {
				t.Errorf("expected stderr to contain %s, got %s", tt.expectedStderr, stderr)
			}

			if exitcode != tt.expectedExitCode {
				t.Errorf("expected exitcode %d, got %d", tt.expectedExitCode, exitcode)
			}

			if !errors.Is(err, tt.expectedError) {
				t.Errorf("expected error (%v), got (%v)", tt.expectedError, err)
			}
		})
	}
}
