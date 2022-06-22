//go:build !windows
// +build !windows

package tasks

func CreateSchedTask(st SchedTask) (bool, error) {
	return true, nil
}

func DeleteSchedTask(name string) error {
	return nil
}

func ListSchedTasks() ([]string, error) {
	return []string{}, nil
}