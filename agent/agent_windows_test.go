package agent

import (
	"errors"
	"strings"
	"testing"
)

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
			code: "Write-Output \"test\"",
			shell: "powershell",
			args: []string{},
			timeout: 30,
			expectedStdout: "test\r\n",
			expectedStderr: "",
			expectedExitcode: 0,
			expectedError: nil,
		},
		{
			name: "Run Bad Script",
			code: "Bad-Command",
			shell: "powershell",
			args: []string{},
			timeout: 30,
			expectedStdout: "",
			expectedStderr: "is not recognized as the name of a cmdlet",
			expectedExitcode: 0,
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