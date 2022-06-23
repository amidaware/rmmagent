//go:build !windows
// +build !windows

package tactical_test

import (
	"testing"

	"github.com/amidaware/rmmagent/agent/tactical"
)

func TestNewAgentConfig(t *testing.T) {
	config := tactical.NewAgentConfig()
	if config.BaseURL == "" {
		t.Fatal("Could not get config")
	}

	t.Logf("Config BaseURL: %s", config.BaseURL)
}

func TestAgentUpdate(t *testing.T) {
	url := "https://github.com/redanthrax/rmmagent/releases/download/v2.0.4/linuxagent"
	result := tactical.AgentUpdate(url, "")
	if !result {
		t.Fatal("Agent update resulted in false")
	}

	t.Log("Agent update resulted in true")
}

func TestAgentUninstall(t *testing.T) {
	result := tactical.AgentUninstall("foo")
	if !result {
		t.Fatal("Agent uninstall resulted in error")
	}

	t.Log("Agent uninstall was true")
}

func TestNixMeshNodeID(t *testing.T) {
	nodeid := tactical.NixMeshNodeID()
	if nodeid == "" {
		t.Fatal("Unable to get mesh node id")
	}

	t.Logf("MeshNodeID: %s", nodeid)
}
