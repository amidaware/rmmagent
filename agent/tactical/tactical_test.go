package tactical

import "testing"

func TestSyncMeshNodeID(t *testing.T) {
	agentConfig := NewAgentConfig()
	if agentConfig.AgentID == "" {
		t.Fatal("Could not get AgentID")
	}

	result := SyncMeshNodeID()
	if !result {
		t.Fatal("SyncMeshNodeID resulted in error")
	}

	t.Log("Synced mesh node id")
}