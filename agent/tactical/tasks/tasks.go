package tasks

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/tactical/api"
	"github.com/amidaware/rmmagent/agent/tactical/config"
)

func RunTask(id int) error {
	config := config.NewAgentConfig()
	data := AutomatedTask{}
	url := fmt.Sprintf("/api/v3/%d/%s/taskrunner/", id, config.AgentID)
	r1, gerr := api.Get(url)
	if gerr != nil {
		return gerr
	}

	if r1.IsError() {
		return nil
	}

	if err := json.Unmarshal(r1.Body(), &data); err != nil {
		return err
	}

	start := time.Now()
	payload := TaskResult{}
	// loop through all task actions
	for _, action := range data.TaskActions {
		action_start := time.Now()
		if action.ActionType == "script" {
			stdout, stderr, retcode, err := system.RunScript(action.Code, action.Shell, action.Args, action.Timeout)

			if err != nil {
			}

			// add text to stdout showing which script ran if more than 1 script
			action_exec_time := time.Since(action_start).Seconds()

			if len(data.TaskActions) > 1 {
				payload.Stdout += fmt.Sprintf("\n------------\nRunning Script: %s. Execution Time: %f\n------------\n\n", action.ScriptName, action_exec_time)
			}

			// save results
			payload.Stdout += stdout
			payload.Stderr += stderr
			payload.RetCode = retcode

			if !data.ContinueOnError && stderr != "" {
				break
			}

		} else if action.ActionType == "cmd" {
			// out[0] == stdout, out[1] == stderr
			out, err := system.CMDShell(action.Shell, []string{}, action.Command, action.Timeout, false)

			if err != nil {
			}

			if len(data.TaskActions) > 1 {
				action_exec_time := time.Since(action_start).Seconds()
				// add text to stdout showing which script ran
				payload.Stdout += fmt.Sprintf("\n------------\nRunning Command: %s. Execution Time: %f\n------------\n\n", action.Command, action_exec_time)
			}

			// save results
			payload.Stdout += out[0]
			payload.Stderr += out[1]
			// no error
			if out[1] == "" {
				payload.RetCode = 0
			} else {
				payload.RetCode = 1

				if !data.ContinueOnError {
					break
				}
			}

		} else {
		}
	}

	payload.ExecTime = time.Since(start).Seconds()
	perr := api.Patch(payload, url)
	if perr != nil {
		return perr
	}

	return nil
}
