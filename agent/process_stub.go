//go:build !darwin
// +build !darwin

/*
Copyright 2026 Amidaware Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import "github.com/elastic/go-sysinfo/types"

func procNameAndMemory(process types.Process, info types.ProcessInfo) (string, uint64, bool) {
	m, _ := process.Memory()
	return info.Name, m.Resident, true
}
