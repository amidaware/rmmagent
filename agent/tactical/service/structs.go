package service

import (
	"github.com/amidaware/rmmagent/agent/disk"
	"github.com/amidaware/rmmagent/agent/services"
	"github.com/amidaware/rmmagent/agent/tasks"
)

type IService struct{}

type WinSvcNats struct {
	Agentid string             `json:"agent_id"`
	WinSvcs []services.Service `json:"services"`
}

type CheckInNats struct {
	Agentid string `json:"agent_id"`
	Version string `json:"version"`
}

type AgentInfoNats struct {
	Agentid      string  `json:"agent_id"`
	Username     string  `json:"logged_in_username"`
	Hostname     string  `json:"hostname"`
	OS           string  `json:"operating_system"`
	Platform     string  `json:"plat"`
	TotalRAM     float64 `json:"total_ram"`
	BootTime     int64   `json:"boot_time"`
	RebootNeeded bool    `json:"needs_reboot"`
	GoArch       string  `json:"goarch"`
}

type WinWMINats struct {
	Agentid string      `json:"agent_id"`
	WMI     interface{} `json:"wmi"`
}

type WinDisksNats struct {
	Agentid string      `json:"agent_id"`
	Disks   []disk.Disk `json:"disks"`
}

type PublicIPNats struct {
	Agentid  string `json:"agent_id"`
	PublicIP string `json:"public_ip"`
}

type NatsMsg struct {
	Func            string            `json:"func"`
	Timeout         int               `json:"timeout"`
	Data            map[string]string `json:"payload"`
	ScriptArgs      []string          `json:"script_args"`
	ProcPID         int32             `json:"procpid"`
	TaskPK          int               `json:"taskpk"`
	ScheduledTask   tasks.SchedTask   `json:"schedtaskpayload"`
	RecoveryCommand string            `json:"recoverycommand"`
	UpdateGUIDs     []string          `json:"guids"`
	ChocoProgName   string            `json:"choco_prog_name"`
	PendingActionPK int               `json:"pending_action_pk"`
	PatchMgmt       bool              `json:"patch_mgmt"`
	ID              int               `json:"id"`
	Code            string            `json:"code"`
}

type RawCMDResp struct {
	Results string `json:"results"`
}

type RunScriptResp struct {
	Stdout   string  `json:"stdout"`
	Stderr   string  `json:"stderr"`
	Retcode  int     `json:"retcode"`
	ExecTime float64 `json:"execution_time"`
	ID       int     `json:"id"`
}
