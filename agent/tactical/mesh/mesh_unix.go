//go:build !windows
// +build !windows

package mesh

import (
	"path/filepath"

	"github.com/amidaware/rmmagent/agent/tactical/config"
)

func GetMeshBinLocation() string {
	ac := config.NewAgentConfig()
	var MeshSysBin string
	if len(ac.CustomMeshDir) > 0 {
		MeshSysBin = filepath.Join(ac.CustomMeshDir, "meshagent")
	} else {
		MeshSysBin = "/opt/tacticalmesh/meshagent"
	}

	return MeshSysBin
}

func RecoverMesh() { }