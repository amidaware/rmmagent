package rpc

import "github.com/amidaware/rmmagent/agent/tasks"

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