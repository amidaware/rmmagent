//go:build !windows
// +build !windows

package events

func GetEventLog(logName string, searchLastDays int) ([]EventLogMsg, error) {
	return []EventLogMsg{}, nil
}