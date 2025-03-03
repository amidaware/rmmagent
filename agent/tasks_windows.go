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
	"path/filepath"
	"strings"
	"time"

	"github.com/amidaware/taskmaster"
	"github.com/rickb777/date/period"
)

type SchedTask struct {
	PK                  int                            `json:"pk"`
	Type                string                         `json:"type"`
	Name                string                         `json:"name"`
	Trigger             string                         `json:"trigger"`
	Enabled             bool                           `json:"enabled"`
	DayInterval         taskmaster.DayInterval         `json:"day_interval"`
	WeekInterval        taskmaster.WeekInterval        `json:"week_interval"`
	DaysOfWeek          taskmaster.DayOfWeek           `json:"days_of_week"`
	DaysOfMonth         taskmaster.DayOfMonth          `json:"days_of_month"`
	RunOnLastDayOfMonth bool                           `json:"run_on_last_day_of_month"`
	MonthsOfYear        taskmaster.Month               `json:"months_of_year"`
	WeeksOfMonth        taskmaster.Week                `json:"weeks_of_month"`
	StartYear           int                            `json:"start_year"`
	StartMonth          time.Month                     `json:"start_month"`
	StartDay            int                            `json:"start_day"`
	StartHour           int                            `json:"start_hour"`
	StartMinute         int                            `json:"start_min"`
	ExpireYear          int                            `json:"expire_year"`
	ExpireMonth         time.Month                     `json:"expire_month"`
	ExpireDay           int                            `json:"expire_day"`
	ExpireHour          int                            `json:"expire_hour"`
	ExpireMinute        int                            `json:"expire_min"`
	RandomDelay         period.Period                  `json:"random_delay"`
	RepetitionInterval  period.Period                  `json:"repetition_interval"`
	RepetitionDuration  period.Period                  `json:"repetition_duration"`
	StopAtDurationEnd   bool                           `json:"stop_at_duration_end"`
	Path                string                         `json:"path"`
	WorkDir             string                         `json:"workdir"`
	Args                string                         `json:"args"`
	TaskPolicy          taskmaster.TaskInstancesPolicy `json:"multiple_instances"`
	RunASAPAfterMissed  bool                           `json:"start_when_available"`
	DeleteAfter         bool                           `json:"delete_expired_task_after"`
	Overwrite           bool                           `json:"overwrite_task"`
}

func (a *Agent) CreateSchedTask(st SchedTask) (bool, error) {
	a.Logger.Debugf("%+v\n", st)
	conn, err := taskmaster.Connect()
	if err != nil {
		a.Logger.Errorln(err)
		return false, err
	}
	defer conn.Disconnect()

	var trigger taskmaster.Trigger
	var action taskmaster.ExecAction
	var tasktrigger taskmaster.TaskTrigger

	var now = time.Now()
	switch st.Trigger {
	case "manual":
		tasktrigger = taskmaster.TaskTrigger{
			Enabled:       st.Enabled,
			StartBoundary: now,
		}
	case "onboarding":
		tasktrigger = taskmaster.TaskTrigger{
			Enabled: st.Enabled,
		}
	default:
		tasktrigger = taskmaster.TaskTrigger{
			Enabled:       st.Enabled,
			StartBoundary: time.Date(st.StartYear, st.StartMonth, st.StartDay, st.StartHour, st.StartMinute, 0, 0, now.Location()),
			RepetitionPattern: taskmaster.RepetitionPattern{
				RepetitionInterval: st.RepetitionInterval,
				RepetitionDuration: st.RepetitionDuration,
				StopAtDurationEnd:  st.StopAtDurationEnd,
			},
		}
	}

	if st.ExpireMinute != 0 {
		tasktrigger.EndBoundary = time.Date(st.ExpireYear, st.ExpireMonth, st.ExpireDay, st.ExpireHour, st.ExpireMinute, 0, 0, now.Location())
	}

	var path, workdir, args string
	def := conn.NewTaskDefinition()

	switch st.Trigger {
	case "runonce":
		trigger = taskmaster.TimeTrigger{
			TaskTrigger: tasktrigger,
			RandomDelay: st.RandomDelay,
		}
	case "onboarding":
		trigger = taskmaster.RegistrationTrigger{
			TaskTrigger: tasktrigger,
			Delay:       st.RandomDelay,
		}

	case "daily":
		trigger = taskmaster.DailyTrigger{
			TaskTrigger: tasktrigger,
			DayInterval: st.DayInterval,
			RandomDelay: st.RandomDelay,
		}

	case "weekly":
		trigger = taskmaster.WeeklyTrigger{
			TaskTrigger:  tasktrigger,
			DaysOfWeek:   st.DaysOfWeek,
			WeekInterval: st.WeekInterval,
			RandomDelay:  st.RandomDelay,
		}

	case "monthly":
		trigger = taskmaster.MonthlyTrigger{
			TaskTrigger:         tasktrigger,
			DaysOfMonth:         st.DaysOfMonth,
			MonthsOfYear:        st.MonthsOfYear,
			RandomDelay:         st.RandomDelay,
			RunOnLastDayOfMonth: st.RunOnLastDayOfMonth,
		}

	case "monthlydow":
		trigger = taskmaster.MonthlyDOWTrigger{
			TaskTrigger:  tasktrigger,
			DaysOfWeek:   st.DaysOfWeek,
			MonthsOfYear: st.MonthsOfYear,
			WeeksOfMonth: st.WeeksOfMonth,
			RandomDelay:  st.RandomDelay,
		}

	case "manual":
		trigger = taskmaster.TimeTrigger{
			TaskTrigger: tasktrigger,
		}
	}

	def.AddTrigger(trigger)

	switch st.Type {
	case "rmm":
		path = winExeName
		workdir = a.ProgramDir
		args = fmt.Sprintf("-m taskrunner -p %d", st.PK)
	case "schedreboot":
		path = "shutdown.exe"
		workdir = filepath.Join(os.Getenv("SYSTEMROOT"), "System32")
		args = `/r /t 5 /f /c "Reboot scheduled by RMM agent." /d p:0:0`
	case "custom":
		path = st.Path
		workdir = st.WorkDir
		args = st.Args
	}

	action = taskmaster.ExecAction{
		Path:       path,
		WorkingDir: workdir,
		Args:       args,
	}
	def.AddAction(action)

	def.Principal.RunLevel = taskmaster.TASK_RUNLEVEL_HIGHEST
	def.Principal.LogonType = taskmaster.TASK_LOGON_SERVICE_ACCOUNT
	def.Principal.UserID = "SYSTEM"
	def.Settings.AllowDemandStart = true
	def.Settings.AllowHardTerminate = true
	def.Settings.DontStartOnBatteries = false
	def.Settings.Enabled = st.Enabled
	def.Settings.StopIfGoingOnBatteries = false
	def.Settings.WakeToRun = true
	def.Settings.MultipleInstances = st.TaskPolicy

	if st.DeleteAfter {
		def.Settings.DeleteExpiredTaskAfter = "PT15M"
	}

	if st.RunASAPAfterMissed {
		def.Settings.StartWhenAvailable = true
	}

	_, success, err := conn.CreateTask(fmt.Sprintf("\\%s", st.Name), def, st.Overwrite)
	if err != nil {
		a.Logger.Errorln(err)
		return false, err
	}

	return success, nil
}

func DeleteSchedTask(name string) error {
	conn, err := taskmaster.Connect()
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	err = conn.DeleteTask(fmt.Sprintf("\\%s", name))
	if err != nil {
		return err
	}
	return nil
}

// CleanupSchedTasks removes all tacticalrmm sched tasks during uninstall
func CleanupSchedTasks() {
	conn, err := taskmaster.Connect()
	if err != nil {
		return
	}
	defer conn.Disconnect()

	tasks, err := conn.GetRegisteredTasks()
	if err != nil {
		return
	}

	for _, task := range tasks {
		if strings.HasPrefix(task.Name, "TacticalRMM_") {
			conn.DeleteTask(fmt.Sprintf("\\%s", task.Name))
		}
	}
	tasks.Release()
}

func ListSchedTasks() []string {
	ret := make([]string, 0)

	conn, err := taskmaster.Connect()
	if err != nil {
		return ret
	}
	defer conn.Disconnect()

	tasks, err := conn.GetRegisteredTasks()
	if err != nil {
		return ret
	}

	for _, task := range tasks {
		ret = append(ret, task.Name)
	}
	tasks.Release()
	return ret
}
