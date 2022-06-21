package checks

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/tactical/api"
	"github.com/amidaware/rmmagent/agent/utils"
	ps "github.com/elastic/go-sysinfo"
	"github.com/go-resty/resty/v2"
)

func CheckRunner(agentID string) error {
	sleepDelay := utils.RandRange(14, 22)
	time.Sleep(time.Duration(sleepDelay) * time.Second)
	for {
		interval, err := GetCheckInterval(agentID)
		if err == nil && !ChecksRunning() {
			_, err = system.CMD(system.GetProgramEXE(), []string{"-m", "checkrunner"}, 600, false)
			if err != nil {
				return err
			}
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}

	return nil
}

func GetCheckInterval(agentID string) (int, error) {
	r, err := api.GetResult(CheckInfo{}, fmt.Sprintf("/api/v3/%s/checkinterval/", agentID))
	if err != nil {
		return 120, err
	}

	if r.IsError() {
		return 120, fmt.Errorf("checkinterval response code: %v", r.StatusCode())
	}

	interval := r.Result().(*CheckInfo).Interval
	return interval, nil
}

// ChecksRunning prevents duplicate checks from running
// Have to do it this way, can't use atomic because they can run from both rpc and tacticalagent services
func ChecksRunning() bool {
	running := false
	procs, err := ps.Processes()
	if err != nil {
		return running
	}
Out:
	for _, process := range procs {
		p, err := process.Info()
		if err != nil {
			continue
		}
		if p.PID == 0 {
			continue
		}
		if p.Exe != system.GetProgramEXE() {
			continue
		}

		for _, arg := range p.Args {
			if arg == "runchecks" || arg == "checkrunner" {
				running = true
				break Out
			}
		}
	}

	return running
}

func RunChecks(agentID string, force bool) error {
	data := rmm.AllChecks{}
	var url string
	if force {
		url = fmt.Sprintf("/api/v3/%s/runchecks/", agentID)
	} else {
		url = fmt.Sprintf("/api/v3/%s/checkrunner/", agentID)
	}

	r, err := a.rClient.R().Get(url)
	if err != nil {
		//a.Logger.Debugln(err)
		return err
	}

	if r.IsError() {
		//a.Logger.Debugln("Checkrunner response code:", r.StatusCode())
		return nil
	}

	if err := json.Unmarshal(r.Body(), &data); err != nil {
		//a.Logger.Debugln(err)
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
				utils.RandomCheckDelay()
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
