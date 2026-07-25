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
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	rmm "github.com/amidaware/rmmagent/shared"
	nats "github.com/nats-io/nats.go"
	"github.com/ugorji/go/codec"
)

type NatsMsg struct {
	Func                   string            `json:"func"`
	Timeout                int               `json:"timeout"`
	Data                   map[string]string `json:"payload"`
	ScriptArgs             []string          `json:"script_args"`
	ProcPID                int32             `json:"procpid"`
	TaskPK                 int               `json:"taskpk"`
	ScheduledTask          SchedTask         `json:"schedtaskpayload"`
	RecoveryCommand        string            `json:"recoverycommand"`
	UpdateGUIDs            []string          `json:"guids"`
	ChocoProgName          string            `json:"choco_prog_name"`
	PendingActionPK        int               `json:"pending_action_pk"`
	PatchMgmt              bool              `json:"patch_mgmt"`
	ID                     int               `json:"id"`
	Code                   string            `json:"code"`
	RunAsUser              bool              `json:"run_as_user"`
	EnvVars                []string          `json:"env_vars"`
	NushellEnableConfig    bool              `json:"nushell_enable_config"`
	DenoDefaultPermissions string            `json:"deno_default_permissions"`
	Stream                 bool              `json:"stream"`
}

var (
	agentUpdateLocker      uint32
	getWinUpdateLocker     uint32
	installWinUpdateLocker uint32
)

func (a *Agent) RunRPC() {
	a.Logger.Infoln("Agent service started")

	opts := a.setupNatsOptions()
	nc, err := nats.Connect(a.NatsServer, opts...)
	a.Logger.Debugf("%+v\n", nc)
	a.Logger.Debugf("%+v\n", nc.Opts)
	if err != nil {
		a.Logger.Fatalln("RunRPC() nats.Connect()", err)
	}

	go a.RunAsService(nc)

	var wg sync.WaitGroup
	wg.Add(1)

	nc.Subscribe(a.AgentID, func(msg *nats.Msg) {
		var payload *NatsMsg
		var mh codec.MsgpackHandle
		mh.RawToString = true

		dec := codec.NewDecoderBytes(msg.Data, &mh)
		if err := dec.Decode(&payload); err != nil {
			a.Logger.Errorln(err)
			return
		}

		switch payload.Func {
		case "ping":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				a.Logger.Debugln("pong")
				ret.Encode("pong")
				msg.Respond(resp)
			}()

		case "patchmgmt":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				err := a.PatchMgmnt(p.PatchMgmt)
				if err != nil {
					a.Logger.Errorln("PatchMgmnt:", err.Error())
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
				success, err := a.CreateSchedTask(p.ScheduledTask)
				if err != nil {
					a.Logger.Errorln(err.Error())
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
				err := DeleteSchedTask(p.ScheduledTask.Name)
				if err != nil {
					a.Logger.Errorln(err.Error())
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
				tasks := ListSchedTasks()
				a.Logger.Debugln(tasks)
				ret.Encode(tasks)
				msg.Respond(resp)
			}()

		case "eventlog":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				days, _ := strconv.Atoi(p.Data["days"])
				evtLog := a.GetEventLog(p.Data["logname"], days)
				a.Logger.Debugln(evtLog)
				ret.Encode(evtLog)
				msg.Respond(resp)
			}(payload)

		case "procs":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				procs := a.GetProcsRPC()
				a.Logger.Debugln(procs)
				ret.Encode(procs)
				msg.Respond(resp)
			}()

		case "killproc":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				err := KillProc(p.ProcPID)
				if err != nil {
					ret.Encode(err.Error())
					a.Logger.Debugln(err.Error())
				} else {
					ret.Encode("ok")
				}
				msg.Respond(resp)
			}(payload)

		case "rawcmd":
			go func(p *NatsMsg) {
				var resp []byte
				var resultData rmm.RawCMDResp
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

				switch runtime.GOOS {
				case "windows":
					cmdID := ""
					if val, ok := p.Data["cmd_id"]; ok {
						cmdID = val
					}
					out, _ := CMDShell(p.Data["shell"], []string{}, p.Data["command"], p.Timeout, false, p.RunAsUser, p.Stream, &a.AgentID, &cmdID, nc)
					a.Logger.Debugln(out)
					if out[1] != "" {
						ret.Encode(out[1])
						resultData.Results = out[1]
					} else {
						ret.Encode(out[0])
						resultData.Results = out[0]
					}
				default:
					opts := a.NewCMDOpts()
					opts.Shell = p.Data["shell"]
					opts.Command = p.Data["command"]
					opts.Timeout = time.Duration(p.Timeout)
					opts.Stream = p.Stream
					opts.Nc = nc
					cmdID := ""
					if val, ok := p.Data["cmd_id"]; ok {
						cmdID = val
					}
					opts.CmdID = cmdID
					out := a.CmdV2(opts)
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
					a.rClient.R().SetBody(resultData).Patch(fmt.Sprintf("/api/v3/%d/%s/histresult/", p.ID, a.AgentID))
				}
			}(payload)

		case "registry_browse":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				// p.Data is map[string]string
				path := p.Data["path"]
				pageStr := p.Data["page"]
				pageSizeStr := p.Data["page_size"]
				page := 1
				pageSize := 200

				if pageStr != "" {
					if v, err := strconv.Atoi(pageStr); err == nil {
						page = v
					}
				}
				if pageSizeStr != "" {
					if v, err := strconv.Atoi(pageSizeStr); err == nil {
						pageSize = v
					}
				}

				if path == "" {
					_ = ret.Encode(map[string]interface{}{"error": "missing path"})
					msg.Respond(resp)
					return
				}

				subkeys, values, hasMore, err := BrowseRegistry(path, page, pageSize)
				if err != nil {
					_ = ret.Encode(map[string]interface{}{"error": err.Error()})
					msg.Respond(resp)
					return
				}

				_ = ret.Encode(map[string]interface{}{
					"path":     path,
					"subkeys":  subkeys,
					"values":   values,
					"has_more": hasMore,
				})
				msg.Respond(resp)
			}(payload)

		case "registry_create_key":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

				path, ok := p.Data["path"]
				if !ok || strings.TrimSpace(path) == "" {
					_ = ret.Encode(map[string]interface{}{"error": "Missing or empty path"})
					msg.Respond(resp)
					return
				}

				err := CreateRegistryKey(path)
				if err != nil {
					_ = ret.Encode(map[string]interface{}{"error": err.Error()})
				} else {
					_ = ret.Encode(map[string]interface{}{"success": true})
				}
				msg.Respond(resp)
			}(payload)

		case "registry_delete_key":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

				path, ok := p.Data["path"]
				if !ok || strings.TrimSpace(path) == "" {
					_ = ret.Encode(map[string]interface{}{"error": "Missing or empty path"})
					msg.Respond(resp)
					return
				}

				err := DeleteRegistryKey(path)
				if err != nil {
					_ = ret.Encode(map[string]interface{}{"error": err.Error()})
				} else {
					_ = ret.Encode(map[string]interface{}{"success": true})
				}

				msg.Respond(resp)
			}(payload)

		case "registry_rename_key":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

				oldPath, ok1 := p.Data["old_path"]
				newPath, ok2 := p.Data["new_path"]
				if !ok1 || !ok2 || strings.TrimSpace(oldPath) == "" || strings.TrimSpace(newPath) == "" {
					_ = ret.Encode(map[string]interface{}{"error": "Both old_path and new_path are required"})
					msg.Respond(resp)
					return
				}

				if oldPath == newPath {
					_ = ret.Encode(map[string]interface{}{"error": "Old and new path cannot be the same"})
					msg.Respond(resp)
					return
				}

				err := RenameRegistryKey(oldPath, newPath)
				if err != nil {
					_ = ret.Encode(map[string]interface{}{"error": err.Error()})
				} else {
					_ = ret.Encode(map[string]interface{}{"success": true})
				}
				msg.Respond(resp)
			}(payload)

		case "registry_create_value":
			go func(payload *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

				path, ok := payload.Data["path"]
				if !ok || strings.TrimSpace(path) == "" {
					_ = ret.Encode(map[string]interface{}{"error": "Missing path"})
					msg.Respond(resp)
					return
				}

				valName := ""
				if n, ok := payload.Data["name"]; ok {
					valName = n
				}

				valType, ok := payload.Data["type"]
				if !ok || strings.TrimSpace(valType) == "" {
					_ = ret.Encode(map[string]interface{}{"error": "Missing value type"})
					msg.Respond(resp)
					return
				}

				var valData interface{}
				if d, ok := payload.Data["data"]; ok {
					valData = d
				}

				created, err := CreateRegistryValue(path, valName, valType, valData)
				if err != nil {
					_ = ret.Encode(map[string]interface{}{"error": err.Error()})
					msg.Respond(resp)
					return
				}

				_ = ret.Encode(map[string]interface{}{
					"status": "success",
					"name":   created["name"],
					"type":   created["type"],
					"data":   created["data"],
				})
				msg.Respond(resp)
			}(payload)

		case "registry_delete_value":
			go func(payload *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

				path, ok := payload.Data["path"]
				if !ok || strings.TrimSpace(path) == "" {
					_ = ret.Encode(map[string]interface{}{"error": "Missing path"})
					msg.Respond(resp)
					return
				}

				valName, ok := payload.Data["name"]
				if !ok || strings.TrimSpace(valName) == "" {
					_ = ret.Encode(map[string]interface{}{"error": "Missing value name"})
					msg.Respond(resp)
					return
				}

				err := DeleteRegistryValue(path, valName)
				if err != nil {
					_ = ret.Encode(map[string]interface{}{"error": err.Error()})
					msg.Respond(resp)
					return
				}

				_ = ret.Encode(map[string]interface{}{
					"status": "success",
					"name":   valName,
				})
				msg.Respond(resp)
			}(payload)

		case "registry_rename_value":
			go func(payload *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

				path, ok := payload.Data["path"]
				if !ok || strings.TrimSpace(path) == "" {
					_ = ret.Encode(map[string]interface{}{"error": "Missing path"})
					msg.Respond(resp)
					return
				}

				oldName, ok := payload.Data["old_name"]
				if !ok || strings.TrimSpace(oldName) == "" {
					_ = ret.Encode(map[string]interface{}{"error": "Missing old_name"})
					msg.Respond(resp)
					return
				}

				newName, ok := payload.Data["new_name"]
				if !ok || strings.TrimSpace(newName) == "" {
					_ = ret.Encode(map[string]interface{}{"error": "Missing new_name"})
					msg.Respond(resp)
					return
				}

				renamed, err := RenameRegistryValue(path, oldName, newName)
				if err != nil {
					_ = ret.Encode(map[string]interface{}{"error": err.Error()})
					msg.Respond(resp)
					return
				}

				_ = ret.Encode(map[string]interface{}{
					"status":   "success",
					"new_name": renamed,
				})
				msg.Respond(resp)
			}(payload)

		case "registry_modify_value":
			go func(payload *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

				path, ok := payload.Data["path"]
				if !ok || strings.TrimSpace(path) == "" {
					_ = ret.Encode(map[string]interface{}{"error": "Missing path"})
					msg.Respond(resp)
					return
				}

				valName, ok := payload.Data["name"]
				if !ok || strings.TrimSpace(valName) == "" {
					_ = ret.Encode(map[string]interface{}{"error": "Missing value name"})
					msg.Respond(resp)
					return
				}

				valType, ok := payload.Data["type"]
				if !ok || strings.TrimSpace(valType) == "" {
					_ = ret.Encode(map[string]interface{}{"error": "Missing value type"})
					msg.Respond(resp)
					return
				}

				valData := payload.Data["data"]

				updated, err := ModifyRegistryValue(path, valName, valType, valData)
				if err != nil {
					_ = ret.Encode(map[string]interface{}{"error": err.Error()})
					msg.Respond(resp)
					return
				}

				_ = ret.Encode(map[string]interface{}{
					"status": "success",
					"name":   updated["name"],
					"type":   updated["type"],
					"data":   updated["data"],
				})
				msg.Respond(resp)
			}(payload)

		case "winservices":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				svcs := a.GetServices()
				a.Logger.Debugln(svcs)
				ret.Encode(svcs)
				msg.Respond(resp)
			}()

		case "winsvcdetail":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				svc := a.GetServiceDetail(p.Data["name"])
				a.Logger.Debugln(svc)
				ret.Encode(svc)
				msg.Respond(resp)
			}(payload)

		case "winsvcaction":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				retData := a.ControlService(p.Data["name"], p.Data["action"])
				a.Logger.Debugln(retData)
				ret.Encode(retData)
				msg.Respond(resp)
			}(payload)

		case "editwinsvc":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				retData := a.EditService(p.Data["name"], p.Data["startType"])
				a.Logger.Debugln(retData)
				ret.Encode(retData)
				msg.Respond(resp)
			}(payload)

		case "runscript":
			go func(p *NatsMsg) {
				var resp []byte
				var retData string
				var resultData rmm.RunScriptResp
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				start := time.Now()
				stdout, stderr, retcode, err := a.RunScript(p.Data["code"], p.Data["shell"], p.ScriptArgs, p.Timeout, p.RunAsUser, p.EnvVars, p.NushellEnableConfig, p.DenoDefaultPermissions)
				resultData.ExecTime = time.Since(start).Seconds()
				resultData.ID = p.ID

				if err != nil {
					a.Logger.Debugln(err)
					retData = err.Error()
					resultData.Retcode = 1
					resultData.Stderr = err.Error()
				} else {
					retData = stdout + stderr // to keep backwards compat
					resultData.Retcode = retcode
					resultData.Stdout = stdout
					resultData.Stderr = stderr
				}
				a.Logger.Debugln(retData)
				ret.Encode(retData)
				msg.Respond(resp)
				if p.ID != 0 {
					results := map[string]interface{}{"script_results": resultData}
					a.rClient.R().SetBody(results).Patch(fmt.Sprintf("/api/v3/%d/%s/histresult/", p.ID, a.AgentID))
				}
			}(payload)

		case "runscriptfull":
			go func(p *NatsMsg) {
				var resp []byte
				var retData rmm.RunScriptResp
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				start := time.Now()
				stdout, stderr, retcode, err := a.RunScript(p.Data["code"], p.Data["shell"], p.ScriptArgs, p.Timeout, p.RunAsUser, p.EnvVars, p.NushellEnableConfig, p.DenoDefaultPermissions)

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
				a.Logger.Debugln(retData)
				ret.Encode(retData)
				msg.Respond(resp)
				if p.ID != 0 {
					results := map[string]interface{}{"script_results": retData}
					a.rClient.R().SetBody(results).Patch(fmt.Sprintf("/api/v3/%d/%s/histresult/", p.ID, a.AgentID))
				}
			}(payload)

		case "recover":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

				switch p.Data["mode"] {
				case "mesh":
					a.Logger.Debugln("Recovering mesh")
					a.RecoverMesh()
				}

				ret.Encode("ok")
				msg.Respond(resp)
			}(payload)
		case "softwarelist":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				sw := a.GetInstalledSoftware()
				a.Logger.Debugln(sw)
				ret.Encode(sw)
				msg.Respond(resp)
			}()

		case "shutdown":
			go func() {
				a.Logger.Debugln("Scheduling immediate shutdown")
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				ret.Encode("ok")
				msg.Respond(resp)
				if runtime.GOOS == "windows" {
					CMD("shutdown.exe", []string{"/s", "/t", "5", "/f"}, 15, false)
				} else {
					opts := a.NewCMDOpts()
					opts.Command = "shutdown -h now"
					a.CmdV2(opts)
				}
			}()

		case "rebootnow":
			go func() {
				a.Logger.Debugln("Scheduling immediate reboot")
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				ret.Encode("ok")
				msg.Respond(resp)
				if runtime.GOOS == "windows" {
					CMD("shutdown.exe", []string{"/r", "/t", "5", "/f"}, 15, false)
				} else {
					opts := a.NewCMDOpts()
					opts.Command = "reboot"
					a.CmdV2(opts)
				}
			}()
		case "needsreboot":
			go func() {
				a.Logger.Debugln("Checking if reboot needed")
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				out, err := a.SystemRebootRequired()
				if err == nil {
					a.Logger.Debugln("Reboot needed:", out)
					ret.Encode(out)
				} else {
					a.Logger.Debugln("Error checking if reboot needed:", err)
					ret.Encode(false)
				}
				msg.Respond(resp)
			}()
		case "sysinfo":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				a.Logger.Debugln("Getting sysinfo with WMI")
				modes := []string{"agent-agentinfo", "agent-disks", "agent-wmi", "agent-publicip"}
				for _, m := range modes {
					a.NatsMessage(nc, m)
				}
				ret.Encode("ok")
				msg.Respond(resp)
			}()
		case "wmi":
			go func() {
				a.Logger.Debugln("Sending WMI")
				a.NatsMessage(nc, "agent-wmi")
			}()
		case "cpuloadavg":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				a.Logger.Debugln("Getting CPU Load Avg")
				loadAvg := a.GetCPULoadAvg()
				a.Logger.Debugln("CPU Load Avg:", loadAvg)
				ret.Encode(loadAvg)
				msg.Respond(resp)
			}()
		case "runchecks":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				if runtime.GOOS == "windows" {
					if a.ChecksRunning() {
						ret.Encode("busy")
						msg.Respond(resp)
						a.Logger.Debugln("Checks are already running, please wait")
					} else {
						ret.Encode("ok")
						msg.Respond(resp)
						a.Logger.Debugln("Running checks")
						_, checkerr := CMD(a.EXE, []string{"-m", "runchecks"}, 600, false)
						if checkerr != nil {
							a.Logger.Errorln("RPC RunChecks", checkerr)
						}
					}
				} else {
					ret.Encode("ok")
					msg.Respond(resp)
					a.Logger.Debugln("Running checks")
					a.RunChecks(true)
				}

			}()
		case "runtask":
			go func(p *NatsMsg) {
				a.Logger.Debugln("Running task")
				a.RunTask(p.TaskPK, false)
			}(payload)

		case "publicip":
			go func() {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				ret.Encode(a.PublicIP())
				msg.Respond(resp)
			}()
		case "installpython":
			go a.GetPython(true)
		case "installnushell":
			go a.InstallNushell(true)
		case "installdeno":
			go a.InstallDeno(true)
		case "installchoco":
			go a.InstallChoco()
		case "installwithchoco":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				ret.Encode("ok")
				msg.Respond(resp)
				out, _ := a.InstallWithChoco(p.ChocoProgName)
				results := map[string]string{"results": out}
				url := fmt.Sprintf("/api/v4/%s/%d/chocoresult/", a.AgentID, p.PendingActionPK)
				a.rClient.R().SetBody(results).Patch(url)
			}(payload)
		case "getwinupdates":
			go func() {
				if !atomic.CompareAndSwapUint32(&getWinUpdateLocker, 0, 1) {
					a.Logger.Debugln("Already checking for windows updates")
				} else {
					a.Logger.Debugln("Checking for windows updates")
					defer atomic.StoreUint32(&getWinUpdateLocker, 0)
					a.GetWinUpdates()
				}
			}()
		case "installwinupdates":
			go func(p *NatsMsg) {
				if !atomic.CompareAndSwapUint32(&installWinUpdateLocker, 0, 1) {
					a.Logger.Debugln("Already installing windows updates")
				} else {
					a.Logger.Debugln("Installing windows updates", p.UpdateGUIDs)
					defer atomic.StoreUint32(&installWinUpdateLocker, 0)
					a.InstallUpdates(p.UpdateGUIDs)
				}
			}(payload)
		case "agentupdate":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				if !atomic.CompareAndSwapUint32(&agentUpdateLocker, 0, 1) {
					a.Logger.Debugln("Agent update already running")
					ret.Encode("updaterunning")
					msg.Respond(resp)
				} else {
					ret.Encode("ok")
					msg.Respond(resp)
					err := a.AgentUpdate(p.Data["url"], p.Data["inno"], p.Data["version"])
					if err != nil {
						atomic.StoreUint32(&agentUpdateLocker, 0)
						return
					}
					atomic.StoreUint32(&agentUpdateLocker, 0)
					nc.Flush()
					nc.Close()
					a.ControlService(winSvcName, "stop")
					os.Exit(0)
				}
			}(payload)

		case "uninstall":
			go func(p *NatsMsg) {
				var resp []byte
				ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
				ret.Encode("ok")
				msg.Respond(resp)
				a.AgentUninstall(p.Code)
				nc.Flush()
				nc.Close()
				os.Exit(0)
			}(payload)

		case "terminal_start":
			go func(p *NatsMsg) {
				sessionID := p.Data["session_id"]
				shell := p.Data["shell"]
				if sessionID == "" {
					a.Logger.Errorln("terminal_start: missing session_id")
					return
				}
				switch runtime.GOOS {
				case "windows":
					if shell == "" {
						shell = "cmd"
					}
					if err := StartTerminalSessionWindows(a.AgentID, a.ProgramDir, sessionID, shell, p.RunAsUser, nc, a.Logger); err != nil {
						a.Logger.Errorln("terminal_start: StartTerminalSessionWindows:", err)
						SendTerminalError(a.AgentID, sessionID, err.Error(), nc)
					}
				default:
					if shell == "" {
						shell = "/bin/bash"
					}
					if err := a.StartTerminalSession(sessionID, shell, nc); err != nil {
						a.Logger.Errorln("terminal_start: StartTerminalSession:", err)
						a.SendTerminalError(a.AgentID, sessionID, err.Error(), nc)
					}
				}
			}(payload)

		case "terminal_input":
			go func(p *NatsMsg) {
				sessionID := p.Data["session_id"]
				if sessionID == "" {
					a.Logger.Errorln("terminal_input: missing session_id")
					return
				}
				data := p.Data["data"]
				if data == "" {
					return
				}
				switch runtime.GOOS {
				case "windows":
					if err := FeedTerminalInputWindows(sessionID, data); err != nil {
						a.Logger.Errorln("terminal_input: FeedTerminalInputWindows:", err)
					}
				default:
					if err := a.FeedTerminalInput(sessionID, data); err != nil {
						a.Logger.Errorln("terminal_input: FeedTerminalInput:", err)
					}
				}
			}(payload)

		case "terminal_resize":
			go func(p *NatsMsg) {
				sessionID := p.Data["session_id"]
				if sessionID == "" {
					a.Logger.Errorln("terminal_resize: missing session_id")
					return
				}
				rowsStr := p.Data["rows"]
				colsStr := p.Data["cols"]
				rows, err1 := strconv.Atoi(rowsStr)
				cols, err2 := strconv.Atoi(colsStr)
				if err1 != nil || err2 != nil || rows <= 0 || cols <= 0 {
					a.Logger.Debugf("terminal_resize: failed to validate values: os=%s session=%s rows=%q cols=%q", runtime.GOOS, sessionID, rowsStr, colsStr)
					return
				}
				switch runtime.GOOS {
				case "windows":
					if err := ResizeTerminalSessionWindows(sessionID, rows, cols); err != nil {
						a.Logger.Errorln("terminal_resize: ResizeTerminalSessionWindows:", err)
					}
				default:
					if err := a.ResizeTerminalSession(sessionID, rows, cols); err != nil {
						a.Logger.Errorln("terminal_resize: ResizeTerminalSession:", err)
					}
				}
			}(payload)

		case "terminal_kill":
			go func(p *NatsMsg) {
				sessionID := p.Data["session_id"]
				if sessionID == "" {
					a.Logger.Errorln("terminal_kill: missing session_id")
					return
				}
				switch runtime.GOOS {
				case "windows":
					if err := KillTerminalSessionWindows(sessionID); err != nil {
						a.Logger.Errorln("terminal_kill: KillTerminalSessionWindows:", err)
					}
				default:
					if err := a.KillTerminalSession(sessionID); err != nil {
						a.Logger.Errorln("terminal_kill: KillTerminalSession:", err)
					}
				}
			}(payload)
		}
	})
	nc.Flush()

	if err := nc.LastError(); err != nil {
		a.Logger.Errorln(err)
		os.Exit(1)
	}

	wg.Wait()
}
