package service

import (
	"github.com/amidaware/rmmagent/agent/disk"
	"github.com/amidaware/rmmagent/agent/services"
)

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
