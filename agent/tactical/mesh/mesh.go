package mesh

import (
	"errors"
	"strings"

	"github.com/amidaware/rmmagent/agent/system"
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

func GetMeshNodeID() (string, error) {
	out, err := system.CMD(GetMeshBinLocation(), []string{"-nodeid"}, 10, false)
	if err != nil {
		return "", err
	}

	stdout := out[0]
	stderr := out[1]

	if stderr != "" {
		return "", err
	}

	if stdout == "" || strings.Contains(strings.ToLower(utils.StripAll(stdout)), "not defined") {
		return "", errors.New("failed to get mesh node id")
	}

	return stdout, nil
}