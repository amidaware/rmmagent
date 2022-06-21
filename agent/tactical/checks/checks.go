package checks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/amidaware/rmmagent/agent/events"
	"github.com/amidaware/rmmagent/agent/services"
	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/tactical/api"
	"github.com/amidaware/rmmagent/agent/utils"
	ps "github.com/elastic/go-sysinfo"
	"github.com/go-ping/ping"
	"github.com/shirou/gopsutil/disk"
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
	data := AllChecks{}
	var url string
	if force {
		url = fmt.Sprintf("/api/v3/%s/runchecks/", agentID)
	} else {
		url = fmt.Sprintf("/api/v3/%s/checkrunner/", agentID)
	}

	r, err := api.Get(url)
	if err != nil {
		return err
	}

	if r.IsError() {
		return fmt.Errorf("response error: %d", r.StatusCode())
	}

	if err := json.Unmarshal(r.Body(), &data); err != nil {
		return err
	}

	var wg sync.WaitGroup
	eventLogChecks := make([]Check, 0)
	winServiceChecks := make([]Check, 0)
	for _, check := range data.Checks {
		switch check.CheckType {
		case "diskspace":
			wg.Add(1)
			go func(c Check, wg *sync.WaitGroup) {
				defer wg.Done()
				utils.RandomCheckDelay()
				SendDiskCheckResult(DiskCheck(c))
			}(check, &wg)
		case "cpuload":
			wg.Add(1)
			go func(c Check, wg *sync.WaitGroup) {
				defer wg.Done()
				CPULoadCheck(c)
			}(check, &wg)
		case "memory":
			wg.Add(1)
			go func(c Check, wg *sync.WaitGroup) {
				defer wg.Done()
				utils.RandomCheckDelay()
				MemCheck(c)
			}(check, &wg)
		case "ping":
			wg.Add(1)
			go func(c Check, wg *sync.WaitGroup) {
				defer wg.Done()
				utils.RandomCheckDelay()
				SendPingCheckResult(PingCheck(c))
			}(check, &wg)
		case "script":
			wg.Add(1)
			go func(c Check, wg *sync.WaitGroup) {
				defer wg.Done()
				utils.RandomCheckDelay()
				ScriptCheck(c)
			}(check, &wg)
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
		go func(wg *sync.WaitGroup) {
			for _, winSvcCheck := range winServiceChecks {
				defer wg.Done()
				SendWinSvcCheckResult(WinSvcCheck(winSvcCheck))
			}
		}(&wg)
	}

	if len(eventLogChecks) > 0 {
		wg.Add(len(eventLogChecks))
		go func(wg *sync.WaitGroup) {
			for _, evtCheck := range eventLogChecks {
				defer wg.Done()
				EventLogCheck(evtCheck)
			}
		}(&wg)
	}

	wg.Wait()
	return nil
}

func SendDiskCheckResult(payload DiskCheckResult) error {
	err := api.Patch(payload, "/api/v3/checkrunner/")
	if err != nil {
		return err
	}

	return nil
}

func DiskCheck(data Check) (payload DiskCheckResult) {
	payload.ID = data.CheckPK
	usage, err := disk.Usage(data.Disk)
	if err != nil {
		payload.Exists = false
		payload.MoreInfo = fmt.Sprintf("Disk %s does not exist", data.Disk)
	}

	payload.Exists = true
	payload.PercentUsed = usage.UsedPercent
	payload.MoreInfo = fmt.Sprintf("Total: %s, Free: %s", utils.ByteCountSI(usage.Total), utils.ByteCountSI(usage.Free))

	return
}

func CPULoadCheck(data Check) error {
	payload := CPUMemResult{
		ID:      data.CheckPK,
		Percent: system.GetCPULoadAvg(),
	}

	err := api.PostPayload(payload, "/api/v3/checkrunner/")
	if err != nil {
		return err
	}

	return nil
}

func MemCheck(data Check) error {
	host, _ := ps.Host()
	mem, _ := host.Memory()
	percent := (float64(mem.Used) / float64(mem.Total)) * 100

	payload := CPUMemResult{ID: data.CheckPK, Percent: int(math.Round(percent))}
	err := api.PostPayload(payload, "/api/v3/checkrunner/")
	if err != nil {
		return err
	}

	return nil
}

func SendPingCheckResult(payload PingCheckResponse) error {
	err := api.Patch(payload, "/api/v3/checkrunner/")
	if err != nil {
		return err
	}

	return nil
}

func PingCheck(data Check) (payload PingCheckResponse) {
	payload.ID = data.CheckPK

	out, err := DoPing(data.IP)
	if err != nil {
		payload.Status = "failing"
		payload.Output = err.Error()
		return
	}

	payload.Status = out.Status
	payload.Output = out.Output
	return
}

func DoPing(host string) (PingResponse, error) {
	var ret PingResponse
	pinger, err := ping.NewPinger(host)
	if err != nil {
		return ret, err
	}

	var buf bytes.Buffer
	pinger.OnRecv = func(pkt *ping.Packet) {
		fmt.Fprintf(&buf, "%d bytes from %s: icmp_seq=%d time=%v\n",
			pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt)
	}

	pinger.OnFinish = func(stats *ping.Statistics) {
		fmt.Fprintf(&buf, "\n--- %s ping statistics ---\n", stats.Addr)
		fmt.Fprintf(&buf, "%d packets transmitted, %d packets received, %v%% packet loss\n",
			stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
		fmt.Fprintf(&buf, "round-trip min/avg/max/stddev = %v/%v/%v/%v\n",
			stats.MinRtt, stats.AvgRtt, stats.MaxRtt, stats.StdDevRtt)
	}

	pinger.Count = 3
	pinger.Size = 24
	pinger.Interval = time.Second
	pinger.Timeout = 5 * time.Second
	pinger.SetPrivileged(true)

	err = pinger.Run()
	if err != nil {
		return ret, err
	}

	ret.Output = buf.String()

	stats := pinger.Statistics()

	if stats.PacketsRecv == stats.PacketsSent || stats.PacketLoss == 0 {
		ret.Status = "passing"
	} else {
		ret.Status = "failing"
	}

	return ret, nil
}

func ScriptCheck(data Check) error {
	start := time.Now()
	stdout, stderr, retcode, _ := system.RunScript(data.Script.Code, data.Script.Shell, data.ScriptArgs, data.Timeout)

	payload := ScriptCheckResult{
		ID:      data.CheckPK,
		Stdout:  stdout,
		Stderr:  stderr,
		Retcode: retcode,
		Runtime: time.Since(start).Seconds(),
	}

	err := api.Patch(payload, "/api/v3/checkrunner/")
	if err != nil {
		return err
	}

	return nil
}

func SendWinSvcCheckResult(payload WinSvcCheckResult) error {
	err := api.Patch(payload, "/api/v3/checkrunner/")
	if err != nil {
		return err
	}

	return nil
}

func WinSvcCheck(data Check) (payload WinSvcCheckResult) {
	payload.ID = data.CheckPK
	status, err := services.GetServiceStatus(data.ServiceName)
	if err != nil {
		if data.PassNotExist {
			payload.Status = "passing"
		} else {
			payload.Status = "failing"
		}

		payload.MoreInfo = err.Error()
		return
	}

	payload.MoreInfo = fmt.Sprintf("Status: %s", status)
	if status == "running" {
		payload.Status = "passing"
	} else if status == "start_pending" && data.PassStartPending {
		payload.Status = "passing"
	} else {
		if data.RestartIfStopped {
			ret := services.ControlService(data.ServiceName, "start")
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

func EventLogCheck(data Check) error {
	log := make([]events.EventLogMsg, 0)
	evtLog, _ := events.GetEventLog(data.LogName, data.SearchLastDays)

	for _, i := range evtLog {
		if i.EventType == data.EventType {
			if !data.EventIDWildcard && !(int(i.EventID) == data.EventID) {
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

	payload := EventLogCheckResult{ID: data.CheckPK, Log: log}
	err := api.Patch(payload, "/api/v3/checkrunner/")
	if err != nil {
		return err
	}

	return nil
}
