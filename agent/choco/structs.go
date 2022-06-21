package choco

type ChocoInstalled struct {
	AgentID   string `json:"agent_id"`
	Installed bool   `json:"installed"`
}
