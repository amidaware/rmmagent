/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"encoding/json"
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"time"

	rmm "github.com/amidaware/rmmagent/shared"
	ps "github.com/elastic/go-sysinfo"
	"github.com/go-resty/resty/v2"
	"github.com/shirou/gopsutil/v3/disk"
)

func (a *Agent) CheckRunner() {
	sleepDelay := randRange(14, 22)
	a.Logger.Debugf("CheckRunner() init sleeping for %v seconds", sleepDelay)
	time.Sleep(time.Duration(sleepDelay) * time.Second)
	for {
		interval, err := a.GetCheckInterval()
		if err == nil && !a.ChecksRunning() {
			if runtime.GOOS == "windows" {
				_, err = CMD(a.EXE, []string{"-m", "checkrunner"}, 600, false)
				if err != nil {
					a.Logger.Errorln("Checkrunner RunChecks", err)
				}
			} else {
				a.RunChecks(false)
			}
		}
		a.Logger.Debugln("Checkrunner sleeping for", interval)
		time.Sleep(time.Duration(interval) * time.Second)
	}
}

func (a *Agent) GetCheckInterval() (int, error) {
	r, err := a.rClient.R().SetResult(&rmm.CheckInfo{}).Get(fmt.Sprintf("/api/v3/%s/checkinterval/", a.AgentID))
	if err != nil {
		a.Logger.Debugln(err)
		return 120, err
	}
	if r.IsError() {
		a.Logger.Debugln("Checkinterval response code:", r.StatusCode())
		return 120, fmt.Errorf("checkinterval response code: %v", r.StatusCode())
	}
	interval := r.Result().(*rmm.CheckInfo).Interval
	return interval, nil
}

func (a *Agent) RunChecks(force bool) error {
	data := rmm.AllChecks{}
	var url string
	if force {
		url = fmt.Sprintf("/api/v3/%s/runchecks/", a.AgentID)
	} else {
		url = fmt.Sprintf("/api/v3/%s/checkrunner/", a.AgentID)
	}
	r, err := a.rClient.R().Get(url)
	if err != nil {
		a.Logger.Debugln(err)
		return err
	}

	if r.IsError() {
		a.Logger.Debugln("Checkrunner response code:", r.StatusCode())
		return nil
	}

	if err := json.Unmarshal(r.Body(), &data); err != nil {
		a.Logger.Debugln(err)
		return err
	}

	var wg sync.WaitGroup
	eventLogChecks := make([]rmm.Check, 0)
	winServiceChecks := make([]rmm.Check, 0)

	for _, check := range data.Checks {
		switch check.CheckType {
		case "diskspace":
			wg.Add(1)
			go func(c rmm.Check, wg *sync.WaitGroup, r *resty.Client) {
				defer wg.Done()
				randomCheckDelay()
				a.SendDiskCheckResult(a.DiskCheck(c), r)
			}(check, &wg, a.rClient)
		case "cpuload":
			wg.Add(1)
			go func(c rmm.Check, wg *sync.WaitGroup, r *resty.Client) {
				defer wg.Done()
				a.CPULoadCheck(c, r)
			}(check, &wg, a.rClient)
		case "memory":
			wg.Add(1)
			go func(c rmm.Check, wg *sync.WaitGroup, r *resty.Client) {
				defer wg.Done()
				randomCheckDelay()
				a.MemCheck(c, r)
			}(check, &wg, a.rClient)
		case "ping":
			wg.Add(1)
			go func(c rmm.Check, wg *sync.WaitGroup, r *resty.Client) {
				defer wg.Done()
				randomCheckDelay()
				a.SendPingCheckResult(a.PingCheck(c), r)
			}(check, &wg, a.rClient)
		case "script":
			wg.Add(1)
			go func(c rmm.Check, wg *sync.WaitGroup, r *resty.Client) {
				defer wg.Done()
				randomCheckDelay()
				a.ScriptCheck(c, r)
			}(check, &wg, a.rClient)
		case "winsvc":
			winServiceChecks = append(winServiceChecks, check)
		case "eventlog":
			eventLogChecks = append(eventLogChecks, check)
		default:
			continue
		}
	}

	if len(winServiceChecks) > 0 {
		wg.Add(len(winServiceChecks))
		go func(wg *sync.WaitGroup, r *resty.Client) {
			for _, winSvcCheck := range winServiceChecks {
				defer wg.Done()
				a.SendWinSvcCheckResult(a.WinSvcCheck(winSvcCheck), r)
			}
		}(&wg, a.rClient)
	}

	if len(eventLogChecks) > 0 {
		wg.Add(len(eventLogChecks))
		go func(wg *sync.WaitGroup, r *resty.Client) {
			for _, evtCheck := range eventLogChecks {
				defer wg.Done()
				a.EventLogCheck(evtCheck, r)
			}
		}(&wg, a.rClient)
	}
	wg.Wait()
	return nil
}

type ScriptCheckResult struct {
	ID      int     `json:"id"`
	AgentID string  `json:"agent_id"`
	Stdout  string  `json:"stdout"`
	Stderr  string  `json:"stderr"`
	Retcode int     `json:"retcode"`
	Runtime float64 `json:"runtime"`
}

// ScriptCheck runs either bat, powershell or python script
func (a *Agent) ScriptCheck(data rmm.Check, r *resty.Client) {
	start := time.Now()
	stdout, stderr, retcode, _ := a.RunScript(data.Script.Code, data.Script.Shell, data.ScriptArgs, data.Timeout, data.Script.RunAsUser, data.EnvVars, data.NushellEnableConfig, data.DenoDefaultPermissions)

	payload := ScriptCheckResult{
		ID:      data.CheckPK,
		AgentID: a.AgentID,
		Stdout:  stdout,
		Stderr:  stderr,
		Retcode: retcode,
		Runtime: time.Since(start).Seconds(),
	}

	_, err := r.R().SetBody(payload).Patch("/api/v3/checkrunner/")
	if err != nil {
		a.Logger.Debugln(err)
	}
}

func (a *Agent) SendDiskCheckResult(payload DiskCheckResult, r *resty.Client) {
	_, err := r.R().SetBody(payload).Patch("/api/v3/checkrunner/")
	if err != nil {
		a.Logger.Debugln(err)
	}
}

type DiskCheckResult struct {
	ID          int     `json:"id"`
	AgentID     string  `json:"agent_id"`
	MoreInfo    string  `json:"more_info"`
	PercentUsed float64 `json:"percent_used"`
	Exists      bool    `json:"exists"`
}

// DiskCheck checks disk usage
func (a *Agent) DiskCheck(data rmm.Check) (payload DiskCheckResult) {
	payload.ID = data.CheckPK
	payload.AgentID = a.AgentID

	usage, err := disk.Usage(data.Disk)
	if err != nil {
		payload.Exists = false
		payload.MoreInfo = fmt.Sprintf("Disk %s does not exist", data.Disk)
		a.Logger.Debugln("Disk", data.Disk, err)
		return
	}

	payload.Exists = true
	payload.PercentUsed = usage.UsedPercent
	payload.MoreInfo = fmt.Sprintf("Total: %s, Free: %s", ByteCountSI(usage.Total), ByteCountSI(usage.Free))
	return
}

type CPUMemResult struct {
	ID      int    `json:"id"`
	AgentID string `json:"agent_id"`
	Percent int    `json:"percent"`
}

// CPULoadCheck checks avg cpu load
func (a *Agent) CPULoadCheck(data rmm.Check, r *resty.Client) {
	payload := CPUMemResult{ID: data.CheckPK, AgentID: a.AgentID, Percent: a.GetCPULoadAvg()}
	_, err := r.R().SetBody(payload).Patch("/api/v3/checkrunner/")
	if err != nil {
		a.Logger.Debugln(err)
	}
}

// MemCheck checks mem percentage
func (a *Agent) MemCheck(data rmm.Check, r *resty.Client) {
	host, _ := ps.Host()
	mem, _ := host.Memory()
	percent := (float64(mem.Used) / float64(mem.Total)) * 100

	payload := CPUMemResult{ID: data.CheckPK, AgentID: a.AgentID, Percent: int(math.Round(percent))}
	_, err := r.R().SetBody(payload).Patch("/api/v3/checkrunner/")
	if err != nil {
		a.Logger.Debugln(err)
	}
}

type EventLogCheckResult struct {
	ID      int               `json:"id"`
	AgentID string            `json:"agent_id"`
	Log     []rmm.EventLogMsg `json:"log"`
}

func (a *Agent) EventLogCheck(data rmm.Check, r *resty.Client) {
	log := make([]rmm.EventLogMsg, 0)
	evtLog := a.GetEventLog(data.LogName, data.SearchLastDays)

	for _, i := range evtLog {
		if i.EventType == data.EventType {
			if !data.EventIDWildcard && (int(i.EventID) != data.EventID) {
				continue
			}

			if data.EventSource == "" && data.EventMessage == "" {
				if data.EventIDWildcard {
					log = append(log, i)
				} else if int(i.EventID) == data.EventID {
					log = append(log, i)
				} else {
					continue
				}
			}

			if data.EventSource != "" && data.EventMessage != "" {
				if data.EventIDWildcard {
					if strings.Contains(i.Source, data.EventSource) && strings.Contains(i.Message, data.EventMessage) {
						log = append(log, i)
					}
				} else if int(i.EventID) == data.EventID {
					if strings.Contains(i.Source, data.EventSource) && strings.Contains(i.Message, data.EventMessage) {
						log = append(log, i)
					}
				}
				continue
			}

			if data.EventSource != "" && strings.Contains(i.Source, data.EventSource) {
				if data.EventIDWildcard {
					log = append(log, i)
				} else if int(i.EventID) == data.EventID {
					log = append(log, i)
				}
			}

			if data.EventMessage != "" && strings.Contains(i.Message, data.EventMessage) {
				if data.EventIDWildcard {
					log = append(log, i)
				} else if int(i.EventID) == data.EventID {
					log = append(log, i)
				}
			}
		}
	}

	payload := EventLogCheckResult{ID: data.CheckPK, AgentID: a.AgentID, Log: log}
	_, err := r.R().SetBody(payload).Patch("/api/v3/checkrunner/")
	if err != nil {
		a.Logger.Debugln(err)
	}
}

func (a *Agent) SendPingCheckResult(payload rmm.PingCheckResponse, r *resty.Client) {
	_, err := r.R().SetBody(payload).Patch("/api/v3/checkrunner/")
	if err != nil {
		a.Logger.Debugln(err)
	}
}

func (a *Agent) PingCheck(data rmm.Check) (payload rmm.PingCheckResponse) {
	payload.ID = data.CheckPK
	payload.AgentID = a.AgentID

	out, err := DoPing(data.IP)
	if err != nil {
		a.Logger.Debugln("PingCheck:", err)
		payload.Status = "failing"
		payload.Output = err.Error()
		return
	}

	payload.Status = out.Status
	payload.Output = out.Output
	return
}

type WinSvcCheckResult struct {
	ID       int    `json:"id"`
	AgentID  string `json:"agent_id"`
	MoreInfo string `json:"more_info"`
	Status   string `json:"status"`
}

func (a *Agent) SendWinSvcCheckResult(payload WinSvcCheckResult, r *resty.Client) {
	_, err := r.R().SetBody(payload).Patch("/api/v3/checkrunner/")
	if err != nil {
		a.Logger.Debugln(err)
	}
}

func (a *Agent) WinSvcCheck(data rmm.Check) (payload WinSvcCheckResult) {
	payload.ID = data.CheckPK
	payload.AgentID = a.AgentID

	status, err := GetServiceStatus(data.ServiceName)
	if err != nil {
		if data.PassNotExist {
			payload.Status = "passing"
		} else {
			payload.Status = "failing"
		}
		payload.MoreInfo = err.Error()
		a.Logger.Debugln("Service", data.ServiceName, err)
		return
	}

	payload.MoreInfo = fmt.Sprintf("Status: %s", status)
	if status == "running" {
		payload.Status = "passing"
	} else if status == "start_pending" && data.PassStartPending {
		payload.Status = "passing"
	} else {
		if data.RestartIfStopped {
			ret := a.ControlService(data.ServiceName, "start")
			if ret.Success {
				payload.Status = "passing"
				payload.MoreInfo = "Status: running"
			} else {
				payload.Status = "failing"
			}
		} else {
			payload.Status = "failing"
		}
	}
	return
}
