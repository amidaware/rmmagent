package tasks

type SchedTask struct {
	Name    string `json:"name"`
	Minute  int    `json:"minute"`
	Hour    int    `json:"hour"`
	Day     int    `json:"day"`
	Month   int    `json:"month"`
	Weekday int    `json:"weekday"`
	Command string `json:"command"`
}
