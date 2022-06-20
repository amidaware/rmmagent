package events

type EventLogMsg struct {
	Source    string `json:"source"`
	EventType string `json:"eventType"`
	EventID   uint32 `json:"eventID"`
	Message   string `json:"message"`
	Time      string `json:"time"`
	UID       int    `json:"uid"` // for vue
}
