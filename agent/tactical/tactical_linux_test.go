package tactical

import (
	"testing"
)

func TestNewAgentConfig(t *testing.T) {
	config := NewAgentConfig()
	if config.BaseURL == "" {
		t.Fatal("Could not get config")
	}

	t.Logf("Config BaseURL: %s", config.BaseURL)
}

func TestAgentUpdate(t *testing.T) {
	url := "https://github.com/redanthrax/rmmagent/releases/download/v2.0.4/linuxagent"
	result := AgentUpdate(url, "", "v2.0.4")
	if(!result) {
		t.Fatal("Agent update resulted in false")
	}

	t.Log("Agent update resulted in true")
}

func TestAgentUninstall(t *testing.T) {
	result := AgentUninstall("foo")
	if !result {
		t.Fatal("Agent uninstall resulted in error")
	}

	t.Log("Agent uninstall was true")
}

func TestNixMeshNodeID(t *testing.T) {
	nodeid := NixMeshNodeID()
	if nodeid == "" {
		t.Fatal("Unable to get mesh node id")
	}

	t.Logf("MeshNodeID: %s", nodeid)
}

