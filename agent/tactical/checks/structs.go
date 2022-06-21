package checks

import "github.com/amidaware/rmmagent/agent/events"

type CheckInfo struct {
	AgentPK  int `json:"agent"`
	Interval int `json:"check_interval"`
}

type AllChecks struct {
	CheckInfo
	Checks []Check
}

type AssignedTask struct {
	TaskPK  int  `json:"id"`
	Enabled bool `json:"enabled"`
}

type Script struct {
	Shell string `json:"shell"`
	Code  string `json:"code"`
}

type Check struct {
	Script           Script         `json:"script"`
	AssignedTasks    []AssignedTask `json:"assigned_tasks"`
	CheckPK          int            `json:"id"`
	CheckType        string         `json:"check_type"`
	Status           string         `json:"status"`
	Threshold        int            `json:"threshold"`
	Disk             string         `json:"disk"`
	IP               string         `json:"ip"`
	ScriptArgs       []string       `json:"script_args"`
	Timeout          int            `json:"timeout"`
	ServiceName      string         `json:"svc_name"`
	PassStartPending bool           `json:"pass_if_start_pending"`
	PassNotExist     bool           `json:"pass_if_svc_not_exist"`
	RestartIfStopped bool           `json:"restart_if_stopped"`
	LogName          string         `json:"log_name"`
	EventID          int            `json:"event_id"`
	EventIDWildcard  bool           `json:"event_id_is_wildcard"`
	EventType        string         `json:"event_type"`
	EventSource      string         `json:"event_source"`
	EventMessage     string         `json:"event_message"`
	FailWhen         string         `json:"fail_when"`
	SearchLastDays   int            `json:"search_last_days"`
}

type DiskCheckResult struct {
	ID          int     `json:"id"`
	MoreInfo    string  `json:"more_info"`
	PercentUsed float64 `json:"percent_used"`
	Exists      bool    `json:"exists"`
}

type CPUMemResult struct {
	ID      int `json:"id"`
	Percent int `json:"percent"`
}

type PingCheckResponse struct {
	ID      int    `json:"id"`
	AgentID string `json:"agent_id"`
	Status  string `json:"status"`
	Output  string `json:"output"`
}

type PingResponse struct {
	Status string
	Output string
}

type ScriptCheckResult struct {
	ID      int     `json:"id"`
	Stdout  string  `json:"stdout"`
	Stderr  string  `json:"stderr"`
	Retcode int     `json:"retcode"`
	Runtime float64 `json:"runtime"`
}

type WinSvcCheckResult struct {
	ID       int    `json:"id"`
	MoreInfo string `json:"more_info"`
	Status   string `json:"status"`
}

type EventLogCheckResult struct {
	ID  int                  `json:"id"`
	Log []events.EventLogMsg `json:"log"`
}
