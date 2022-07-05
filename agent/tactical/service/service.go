package service

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/amidaware/rmmagent/agent/choco"
	"github.com/amidaware/rmmagent/agent/events"
	"github.com/amidaware/rmmagent/agent/network"
	"github.com/amidaware/rmmagent/agent/patching"
	"github.com/amidaware/rmmagent/agent/services"
	"github.com/amidaware/rmmagent/agent/software"
	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/tactical"
	"github.com/amidaware/rmmagent/agent/tactical/api"
	"github.com/amidaware/rmmagent/agent/tactical/checks"
	"github.com/amidaware/rmmagent/agent/tactical/config"
	"github.com/amidaware/rmmagent/agent/tactical/mesh"
	"github.com/amidaware/rmmagent/agent/tactical/shared"
	ttasks "github.com/amidaware/rmmagent/agent/tactical/tasks"
	"github.com/amidaware/rmmagent/agent/tasks"
	"github.com/amidaware/rmmagent/agent/utils"
	ksvc "github.com/kardianos/service"
	"github.com/nats-io/nats.go"
	"github.com/ugorji/go/codec"
)

var (
	agentUpdateLocker      uint32
	getWinUpdateLocker     uint32
	installWinUpdateLocker uint32
)

var natsCheckin = []string{"agent-hello", "agent-agentinfo", "agent-disks", "agent-winsvc", "agent-publicip", "agent-wmi"}

func RunRPC() error {
	version := tactical.GetVersion()
	config := config.NewAgentConfig()
	go RunAsService(version)
	var wg sync.WaitGroup
	wg.Add(1)
	opts := SetupNatsOptions()
	server := fmt.Sprintf("tls://%s:4222", config.APIURL)
	nc, err := nats.Connect(server, opts...)
	if err != nil {
		return err
	}

	nc.Subscribe(config.AgentID, func(msg *nats.Msg) {
		var payload *NatsMsg
		var mh codec.MsgpackHandle
		mh.RawToString = true

		dec := codec.NewDecoderBytes(msg.Data, &mh)
		if err := dec.Decode(&payload); err != nil {
			return
		}

		switch payload.Func {
		case "ping":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				ret.Encode("pong")
				msg.Respond(resp)
			}()

		case "patchmgmt":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				err := patching.PatchMgmnt(p.PatchMgmt)
				if err != nil {
					ret.Encode(err.Error())
				} else {
					ret.Encode("ok")
				}
				msg.Respond(resp)
			}(payload)

		case "schedtask":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				success, err := tasks.CreateSchedTask(p.ScheduledTask)
				if err != nil {
					ret.Encode(err.Error())
				} else if !success {
					ret.Encode("Something went wrong")
				} else {
					ret.Encode("ok")
				}
				msg.Respond(resp)
			}(payload)

		case "delschedtask":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				err := tasks.DeleteSchedTask(p.ScheduledTask.Name)
				if err != nil {
					ret.Encode(err.Error())
				} else {
					ret.Encode("ok")
				}
				msg.Respond(resp)
			}(payload)

		case "listschedtasks":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				tasks, _ := tasks.ListSchedTasks()
				ret.Encode(tasks)
				msg.Respond(resp)
			}()

		case "eventlog":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				days, _ := strconv.Atoi(p.Data["days"])
				evtLog, _ := events.GetEventLog(p.Data["logname"], days)
				ret.Encode(evtLog)
				msg.Respond(resp)
			}(payload)

		case "procs":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				procs := system.GetProcsRPC()
				ret.Encode(procs)
				msg.Respond(resp)
			}()

		case "killproc":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				err := system.KillProc(p.ProcPID)
				if err != nil {
					ret.Encode(err.Error())
				} else {
					ret.Encode("ok")
				}
				msg.Respond(resp)
			}(payload)

		case "rawcmd":
			go func(p *NatsMsg) {
				var resp []byte
				var resultData RawCMDResp
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

				switch runtime.GOOS {
				case "windows":
					out, _ := system.CMDShell(p.Data["shell"], []string{}, p.Data["command"], p.Timeout, false)
					if out[1] != "" {
						ret.Encode(out[1])
						resultData.Results = out[1]
					} else {
						ret.Encode(out[0])
						resultData.Results = out[0]
					}
				default:
					opts := system.NewCMDOpts()
					opts.Shell = p.Data["shell"]
					opts.Command = p.Data["command"]
					opts.Timeout = time.Duration(p.Timeout)
					out := system.CmdV2(opts)
					tmp := ""
					if len(out.Stdout) > 0 {
						tmp += out.Stdout
					}
					if len(out.Stderr) > 0 {
						tmp += "\n"
						tmp += out.Stderr
					}
					ret.Encode(tmp)
					resultData.Results = tmp
				}

				msg.Respond(resp)
				if p.ID != 0 {
					api.Patch(resultData, fmt.Sprintf("/api/v3/%d/%s/histresult/", p.ID, config.AgentID))
				}
			}(payload)

		case "winservices":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				svcs, _, _ := services.GetServices()
				ret.Encode(svcs)
				msg.Respond(resp)
			}()

		case "winsvcdetail":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				svc := services.GetServiceDetail(p.Data["name"])
				ret.Encode(svc)
				msg.Respond(resp)
			}(payload)

		case "winsvcaction":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				retData := services.ControlService(p.Data["name"], p.Data["action"])
				ret.Encode(retData)
				msg.Respond(resp)
			}(payload)

		case "editwinsvc":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				retData := services.EditService(p.Data["name"], p.Data["startType"])
				ret.Encode(retData)
				msg.Respond(resp)
			}(payload)

		case "runscript":
			go func(p *NatsMsg) {
				var resp []byte
				var retData string
				var resultData RunScriptResp
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				start := time.Now()
				stdout, stderr, retcode, err := system.RunScript(p.Data["code"], p.Data["shell"], p.ScriptArgs, p.Timeout)
				resultData.ExecTime = time.Since(start).Seconds()
				resultData.ID = p.ID

				if err != nil {
					retData = err.Error()
					resultData.Retcode = 1
					resultData.Stderr = err.Error()
				} else {
					retData = stdout + stderr // to keep backwards compat
					resultData.Retcode = retcode
					resultData.Stdout = stdout
					resultData.Stderr = stderr
				}
				//a.Logger.Debugln(retData)
				ret.Encode(retData)
				msg.Respond(resp)
				if p.ID != 0 {
					results := map[string]interface{}{"script_results": resultData}
					api.Patch(results, fmt.Sprintf("/api/v3/%d/%s/histresult/", p.ID, config.AgentID))
				}
			}(payload)

		case "runscriptfull":
			go func(p *NatsMsg) {
				var resp []byte
				var retData RunScriptResp
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				start := time.Now()
				stdout, stderr, retcode, err := system.RunScript(p.Data["code"], p.Data["shell"], p.ScriptArgs, p.Timeout)

				retData.ExecTime = time.Since(start).Seconds()
				if err != nil {
					retData.Stderr = err.Error()
					retData.Retcode = 1
				} else {
					retData.Stdout = stdout
					retData.Stderr = stderr
					retData.Retcode = retcode
				}
				retData.ID = p.ID
				//a.Logger.Debugln(retData)
				ret.Encode(retData)
				msg.Respond(resp)
				if p.ID != 0 {
					results := map[string]interface{}{"script_results": retData}
					api.Patch(results, fmt.Sprintf("/api/v3/%d/%s/histresult/", p.ID, config.AgentID))
				}
			}(payload)

		case "recover":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

				switch p.Data["mode"] {
				case "mesh":
					mesh.RecoverMesh()
				}

				ret.Encode("ok")
				msg.Respond(resp)
			}(payload)
		case "softwarelist":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				sw, _ := software.GetInstalledSoftware()
				ret.Encode(sw)
				msg.Respond(resp)
			}()

		case "rebootnow":
			go func() {
				//a.Logger.Debugln("Scheduling immediate reboot")
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				ret.Encode("ok")
				msg.Respond(resp)
				if runtime.GOOS == "windows" {
					system.CMD("shutdown.exe", []string{"/r", "/t", "5", "/f"}, 15, false)
				} else {
					opts := system.NewCMDOpts()
					opts.Command = "reboot"
					system.CmdV2(opts)
				}
			}()
		case "needsreboot":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				out, err := system.SystemRebootRequired()
				if err == nil {
					ret.Encode(out)
				} else {
					ret.Encode(false)
				}
				msg.Respond(resp)
			}()
		case "sysinfo":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				modes := []string{"agent-agentinfo", "agent-disks", "agent-wmi", "agent-publicip"}
				for _, m := range modes {
					NatsMessage(version, nc, m)
				}
				ret.Encode("ok")
				msg.Respond(resp)
			}()
		case "wmi":
			go func() {
				NatsMessage(version, nc, "agent-wmi")
			}()
		case "cpuloadavg":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				loadAvg := system.GetCPULoadAvg()
				ret.Encode(loadAvg)
				msg.Respond(resp)
			}()
		case "runchecks":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				if runtime.GOOS == "windows" {
					if checks.ChecksRunning() {
						ret.Encode("busy")
						msg.Respond(resp)
					} else {
						ret.Encode("ok")
						msg.Respond(resp)
						_, checkerr := system.CMD(shared.GetProgramBin(), []string{"-m", "runchecks"}, 600, false)
						if checkerr != nil {
						}
					}
				} else {
					ret.Encode("ok")
					msg.Respond(resp)
					checks.RunChecks(config.AgentID, true)
				}

			}()
		case "runtask":
			go func(p *NatsMsg) {
				ttasks.RunTask(p.TaskPK)
			}(payload)

		case "publicip":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				ret.Encode(network.PublicIP(config.Proxy))
				msg.Respond(resp)
			}()
		case "installpython":
			go shared.GetPython(true)
		case "installchoco":
			go choco.InstallChoco()
		case "installwithchoco":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				ret.Encode("ok")
				msg.Respond(resp)
				out, _ := choco.InstallWithChoco(p.ChocoProgName)
				results := map[string]string{"results": out}
				url := fmt.Sprintf("/api/v3/%d/chocoresult/", p.PendingActionPK)
				api.Patch(results, url)
			}(payload)
		case "getwinupdates":
			go func() {
				if !atomic.CompareAndSwapUint32(&getWinUpdateLocker, 0, 1) {
					//a.Logger.Debugln("Already checking for windows updates")
				} else {
					//a.Logger.Debugln("Checking for windows updates")
					defer atomic.StoreUint32(&getWinUpdateLocker, 0)
					patching.GetUpdates()
				}
			}()
		case "installwinupdates":
			go func(p *NatsMsg) {
				if !atomic.CompareAndSwapUint32(&installWinUpdateLocker, 0, 1) {
					//a.Logger.Debugln("Already installing windows updates")
				} else {
					//a.Logger.Debugln("Installing windows updates", p.UpdateGUIDs)
					defer atomic.StoreUint32(&installWinUpdateLocker, 0)
					patching.InstallUpdates(p.UpdateGUIDs)
				}
			}(payload)
		case "agentupdate":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				if !atomic.CompareAndSwapUint32(&agentUpdateLocker, 0, 1) {
					//a.Logger.Debugln("Agent update already running")
					ret.Encode("updaterunning")
					msg.Respond(resp)
				} else {
					ret.Encode("ok")
					msg.Respond(resp)
					tactical.AgentUpdate(p.Data["url"], p.Data["inno"])
					atomic.StoreUint32(&agentUpdateLocker, 0)
					nc.Flush()
					nc.Close()
					os.Exit(0)
				}
			}(payload)

		case "uninstall":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				ret.Encode("ok")
				msg.Respond(resp)
				tactical.AgentUninstall(p.Code)
				nc.Flush()
				nc.Close()
				os.Exit(0)
			}(payload)
		}
	})
	nc.Flush()

	if err := nc.LastError(); err != nil {
		return err
	}

	wg.Wait()

	return nil
}

func RunAsService(version string) {
	var wg sync.WaitGroup
	wg.Add(1)
	go AgentSvc(version)
	go checks.CheckRunner(version)
	wg.Wait()
}

func AgentSvc(version string) error {
	config := config.NewAgentConfig()
	go shared.GetPython(false)
	utils.CreateTRMMTempDir()
	shared.RunMigrations()
	sleepDelay := utils.RandRange(14, 22)
	time.Sleep(time.Duration(sleepDelay) * time.Second)
	opts := SetupNatsOptions()
	server := fmt.Sprintf("tls://%s:4222", config.APIURL)
	nc, err := nats.Connect(server, opts...)
	if err != nil {
		return err
	}

	for _, s := range natsCheckin {
		NatsMessage(version, nc, s)
		time.Sleep(time.Duration(utils.RandRange(100, 400)) * time.Millisecond)
	}

	go mesh.SyncMeshNodeID()

	time.Sleep(time.Duration(utils.RandRange(1, 3)) * time.Second)
	AgentStartup(config.AgentID)
	SendSoftware()

	checkInHelloTicker := time.NewTicker(time.Duration(utils.RandRange(30, 60)) * time.Second)
	checkInAgentInfoTicker := time.NewTicker(time.Duration(utils.RandRange(200, 400)) * time.Second)
	checkInWinSvcTicker := time.NewTicker(time.Duration(utils.RandRange(2400, 3000)) * time.Second)
	checkInPubIPTicker := time.NewTicker(time.Duration(utils.RandRange(300, 500)) * time.Second)
	checkInDisksTicker := time.NewTicker(time.Duration(utils.RandRange(1000, 2000)) * time.Second)
	checkInSWTicker := time.NewTicker(time.Duration(utils.RandRange(2800, 3500)) * time.Second)
	checkInWMITicker := time.NewTicker(time.Duration(utils.RandRange(3000, 4000)) * time.Second)
	syncMeshTicker := time.NewTicker(time.Duration(utils.RandRange(800, 1200)) * time.Second)

	for {
		select {
		case <-checkInHelloTicker.C:
			NatsMessage(version, nc, "agent-hello")
		case <-checkInAgentInfoTicker.C:
			NatsMessage(version, nc, "agent-agentinfo")
		case <-checkInWinSvcTicker.C:
			NatsMessage(version, nc, "agent-winsvc")
		case <-checkInPubIPTicker.C:
			NatsMessage(version, nc, "agent-publicip")
		case <-checkInDisksTicker.C:
			NatsMessage(version, nc, "agent-disks")
		case <-checkInSWTicker.C:
			SendSoftware()
		case <-checkInWMITicker.C:
			NatsMessage(version, nc, "agent-wmi")
		case <-syncMeshTicker.C:
			mesh.SyncMeshNodeID()
		}
	}
}

func SetupNatsOptions() []nats.Option {
	config := config.NewAgentConfig()
	opts := make([]nats.Option, 0)
	opts = append(opts, nats.Name("TacticalRMM"))
	opts = append(opts, nats.UserInfo(config.AgentID, config.Token))
	opts = append(opts, nats.ReconnectWait(time.Second*5))
	opts = append(opts, nats.RetryOnFailedConnect(true))
	opts = append(opts, nats.MaxReconnects(-1))
	opts = append(opts, nats.ReconnectBufSize(-1))
	return opts
}

func DoNatsCheckIn(version string) {
	opts := SetupNatsOptions()
	server := fmt.Sprintf("tls://%s:4222", config.NewAgentConfig().APIURL)
	nc, err := nats.Connect(server, opts...)
	if err != nil {
		return
	}

	for _, s := range natsCheckin {
		time.Sleep(time.Duration(utils.RandRange(100, 400)) * time.Millisecond)
		NatsMessage(version, nc, s)
	}

	nc.Close()
}

func AgentStartup(agentID string) error {
	payload := map[string]interface{}{"agent_id": agentID}
	err := api.PostPayload(payload, "/api/v3/checkin/")
	return err
}

func SendSoftware() error {
	config := config.NewAgentConfig()
	sw, _ := software.GetInstalledSoftware()
	payload := map[string]interface{}{"agent_id": config.AgentID, "software": sw}
	err := api.PostPayload(payload, "/api/v3/software/")
	if err != nil {
		return err
	}

	return nil
}

func (r IService) Start(_ ksvc.Service) error {
	go RunRPC()
	return nil
}

func (r IService) Stop(_ ksvc.Service) error { return nil }
