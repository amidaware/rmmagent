package tactical

import (
	"time"

	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/amidaware/rmmagent/shared"
	"github.com/go-resty/resty/v2"
)

func SyncMeshNodeID() bool {
	id, err := GetMeshNodeID()
	if err != nil {
		//a.Logger.Errorln("SyncMeshNodeID() getMeshNodeID()", err)
		return false
	}

	agentConfig := NewAgentConfig()

	payload := shared.MeshNodeID{
		Func:    "syncmesh",
		Agentid: agentConfig.AgentID,
		NodeID:  utils.StripAll(id),
	}

	client := resty.New()
	client.SetBaseURL(agentConfig.BaseURL)
	client.SetTimeout(15 * time.Second)
	client.SetCloseConnection(true)
	if shared.DEBUG {
		client.SetDebug(true)
	}

	_, err = client.R().SetBody(payload).Post("/api/v3/syncmesh/")
	return err == nil
}
