package install

import "time"

type Installer struct {
	Headers     map[string]string
	RMM         string
	ClientID    int
	SiteID      int
	Description string
	AgentType   string
	Power       bool
	RDP         bool
	Ping        bool
	Token       string
	LocalMesh   string
	Cert        string
	Proxy       string
	Timeout     time.Duration
	SaltMaster  string
	Silent      bool
	NoMesh      bool
	MeshDir     string
	MeshNodeID  string
	Version     string
}

type NewAgentResp struct {
	AgentPK int    `json:"pk"`
	Token   string `json:"token"`
}
