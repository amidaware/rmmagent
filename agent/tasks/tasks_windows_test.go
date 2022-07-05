package tasks_test

//import (
	//"errors"
	//"testing"

	//"github.com/amidaware/rmmagent/agent/tasks"
	//"github.com/amidaware/taskmaster"
//)

//func TestCreateSchedTask(t *testing.T) {
	//testTask := tasks.SchedTask{
		//PK:          0,
		//Name:        "Test Task",
		//Trigger:     "manual",
		//Enabled:     false,
		//Type:        "rmm",
		//TaskPolicy:  taskmaster.TASK_INSTANCES_IGNORE_NEW,
		//DeleteAfter: true,
		//Overwrite:   true,
	//}

	//testTable := []struct {
		//name          string
		//expected      bool
		//expectedError error
	//}{
		//{
			//name:          "Create Sched Task",
			//expected:      true,
			//expectedError: nil,
		//},
	//}

	//for _, tt := range testTable {
		//t.Run(tt.name, func(t *testing.T) {
			//result, err := tasks.CreateSchedTask(testTask)
			//if !result {
				//t.Errorf("Expected %t, got %t", tt.expected, result)
			//}

			//if !errors.Is(tt.expectedError, err) {
				//t.Errorf("expected (%v), got (%v)", tt.expectedError, err)
			//}
		//})
	//}
//}

//func TestListSchedTasks(t *testing.T) {
	//testTable := []struct {
		//name          string
		//expected      []string
		//atLeast       int
		//expectedError error
	//}{
		//{
			//name:          "List Sched Task",
			//expected:      []string{},
			//atLeast:       1,
			//expectedError: nil,
		//},
	//}

	//for _, tt := range testTable {
		//t.Run(tt.name, func(t *testing.T) {
			//result, err := tasks.ListSchedTasks()
			//if len(result) < tt.atLeast {
				//t.Errorf("expect at least %d, got %d", tt.atLeast, len(result))
			//}

			//if !errors.Is(tt.expectedError, err) {
				//t.Errorf("expected (%v), got (%v)", tt.expectedError, err)
			//}
		//})
	//}
//}
