//go:build !windows
// +build !windows

package shared

import (
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	binName    = "tacticalagent"
)

func GetPython(force bool) {}

func RunMigrations() {}

func GetPythonBin() string {
	pybin, err := exec.Command("python", "-c", "import sys; print(sys.executable)").Output()
	if err != nil {
		return ""
	}

	return strings.TrimSuffix(string(pybin), "\n")
}

func GetProgramDirectory() string {
	return "/usr/local/bin"
}

func GetProgramBin() string {
	bin := filepath.Join(GetProgramDirectory(), binName)
	return bin
}