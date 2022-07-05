//go:build !windows
// +build !windows

package software

import (
	"strings"

	"github.com/amidaware/rmmagent/agent/system"
)

func GetInstalledSoftware() ([]Software, error) {
	opts := system.NewCMDOpts()
	opts.Command = "find /usr/share/applications -maxdepth 1 -type f -exec basename {} .desktop \\; | sort"
	result := system.CmdV2(opts)
	softwares := strings.Split(result.Stdout, "\n")
	software := []Software{}
	for _, s := range softwares {
		software = append(software, Software {
			Name: s,
		})
	}

	return software, nil
}