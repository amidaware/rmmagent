package tactical

import (
	"time"

	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/amidaware/rmmagent/shared"
	"github.com/go-resty/resty/v2"
)

func PostRequest(url string, body interface{}, timeout time.Duration) (resty.Response, error) {
	agentConfig := NewAgentConfig()
	client := resty.New()
	client.SetBaseURL(agentConfig.BaseURL)
	client.SetTimeout(timeout * time.Second)
	client.SetCloseConnection(true)
	if len(agentConfig.Proxy) > 0 {
		client.SetProxy(agentConfig.Proxy)
	}

	if shared.DEBUG {
		client.SetDebug(true)
	}

	response, err := client.R().SetBody(body).Post(url)

	return *response, err
}

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

	_, err = PostRequest("/api/v3/syncmesh/", payload, 15)
	return err == nil
}