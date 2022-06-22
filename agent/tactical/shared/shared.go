package shared

import (
	"github.com/amidaware/rmmagent/agent/software"
	"github.com/amidaware/rmmagent/agent/tactical/api"
	"github.com/amidaware/rmmagent/agent/tactical/config"
)

func SendSoftware() error {
	config := config.NewAgentConfig()
	sw, _ := software.GetInstalledSoftware()
	payload := map[string]interface{}{"agent_id": config.AgentID, "software": sw}
	err := api.PostPayload(payload, "/api/v3/software/")
	if err != nil {
		return err
	}

	return nil
}