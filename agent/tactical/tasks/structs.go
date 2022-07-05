package tasks

type AutomatedTask struct {
	ID              int          `json:"id"`
	TaskActions     []TaskAction `json:"task_actions"`
	Enabled         bool         `json:"enabled"`
	ContinueOnError bool         `json:"continue_on_error"`
}

type TaskAction struct {
	ActionType string   `json:"type"`
	Command    string   `json:"command"`
	Shell      string   `json:"shell"`
	ScriptName string   `json:"script_name"`
	Code       string   `json:"code"`
	Args       []string `json:"script_args"`
	Timeout    int      `json:"timeout"`
}

type TaskResult struct {
	Stdout   string  `json:"stdout"`
	Stderr   string  `json:"stderr"`
	RetCode  int     `json:"retcode"`
	ExecTime float64 `json:"execution_time"`
}
