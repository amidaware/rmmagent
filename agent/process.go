/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"fmt"
	"strings"

	rmm "github.com/amidaware/rmmagent/shared"
	ps "github.com/elastic/go-sysinfo"
	gops "github.com/shirou/gopsutil/v3/process"
)

func (a *Agent) GetProcsRPC() []rmm.ProcessMsg {
	ret := make([]rmm.ProcessMsg, 0)

	procs, _ := ps.Processes()
	for i, process := range procs {
		p, err := process.Info()
		if err != nil {
			continue
		}
		if p.PID == 0 {
			continue
		}

		m, _ := process.Memory()
		proc, gerr := gops.NewProcess(int32(p.PID))
		if gerr != nil {
			continue
		}
		cpu, _ := proc.CPUPercent()
		user, _ := proc.Username()

		ret = append(ret, rmm.ProcessMsg{
			Name:     p.Name,
			Pid:      p.PID,
			MemBytes: m.Resident,
			Username: user,
			UID:      i,
			CPU:      fmt.Sprintf("%.1f", cpu),
		})
	}
	return ret
}

func (a *Agent) KillHungUpdates() {
	procs, err := ps.Processes()
	if err != nil {
		return
	}

	for _, process := range procs {
		p, err := process.Info()
		if err != nil {
			continue
		}

		// winagent-v* is deprecated
		if strings.Contains(p.Exe, "winagent-v") {
			a.Logger.Debugln("killing process", p.Exe)
			KillProc(int32(p.PID))
		}

		if strings.Contains(p.Exe, "tacticalagent-v") {
			a.Logger.Debugln("killing process", p.Exe)
			KillProc(int32(p.PID))
		}
	}
}
