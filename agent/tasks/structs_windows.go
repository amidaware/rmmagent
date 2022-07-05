package tasks

import (
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