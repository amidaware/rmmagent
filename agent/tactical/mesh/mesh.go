package mesh

import (
	"github.com/amidaware/rmmagent/agent/tactical/api"
	"github.com/amidaware/rmmagent/agent/tactical/config"
	"github.com/amidaware/rmmagent/agent/utils"
)

func SyncMeshNodeID() error {
	config := config.NewAgentConfig()
	id, err := GetMeshNodeID()
	if err != nil {
		return err
	}

	payload := MeshNodeID{
		Func:    "syncmesh",
		Agentid: config.AgentID,
		NodeID:  utils.StripAll(id),
	}

	err = api.PostPayload(payload, "/api/v3/syncmesh/")
	if err != nil {
		return err
	}

	return nil
}
