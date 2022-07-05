package system

import "time"

type CmdOptions struct {
	Shell        string
	Command      string
	Args         []string
	Timeout      time.Duration
	IsScript     bool
	IsExecutable bool
	Detached     bool
}

type SchedTask struct{ Name string }

type ProcessMsg struct {
	Name     string `json:"name"`
	Pid      int    `json:"pid"`
	MemBytes uint64 `json:"membytes"`
	Username string `json:"username"`
	UID      int    `json:"id"`
	CPU      string `json:"cpu_percent"`
}
