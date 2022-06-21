package mesh

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/tactical/config"
	"github.com/amidaware/rmmagent/agent/utils"
	ps "github.com/elastic/go-sysinfo"
)

// ForceKillMesh kills all mesh agent related processes
func ForceKillMesh() error {
	pids := make([]int, 0)
	procs, err := ps.Processes()
	if err != nil {
		return err
	}

	for _, process := range procs {
		p, err := process.Info()
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(p.Name), "meshagent") {
			pids = append(pids, p.PID)
		}
	}

	for _, pid := range pids {
		if err := system.KillProc(int32(pid)); err != nil {
			return err
		}
	}

	return nil
}

func GetMeshNodeID() (string, error) {
	out, err := system.CMD(getMeshBinLocation(), []string{"-nodeid"}, 10, false)
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

func getMeshBinLocation() string {
	ac := config.NewAgentConfig()
	var MeshSysBin string
	if len(ac.CustomMeshDir) > 0 {
		MeshSysBin = filepath.Join(ac.CustomMeshDir, "MeshAgent.exe")
	} else {
		MeshSysBin = filepath.Join(os.Getenv("ProgramFiles"), "Mesh Agent", "MeshAgent.exe")
	}

	return MeshSysBin
}

func installMesh(meshbin, exe, proxy string) (string, error) {
	var meshNodeID string
	meshInstallArgs := []string{"-fullinstall"}
	if len(proxy) > 0 {
		meshProxy := fmt.Sprintf("--WebProxy=%s", proxy)
		meshInstallArgs = append(meshInstallArgs, meshProxy)
	}

	//a.Logger.Debugln("Mesh install args:", meshInstallArgs)
	meshOut, meshErr := system.CMD(meshbin, meshInstallArgs, int(90), false)
	if meshErr != nil {
		fmt.Println(meshOut[0])
		fmt.Println(meshOut[1])
		fmt.Println(meshErr)
	}

	fmt.Println(meshOut)
	//a.Logger.Debugln("Sleeping for 5")
	time.Sleep(5 * time.Second)

	meshSuccess := false

	for !meshSuccess {
		//a.Logger.Debugln("Getting mesh node id")
		pMesh, pErr := system.CMD(exe, []string{"-nodeid"}, int(30), false)
		if pErr != nil {
			//a.Logger.Errorln(pErr)
			time.Sleep(5 * time.Second)
			continue
		}

		if pMesh[1] != "" {
			//a.Logger.Errorln(pMesh[1])
			time.Sleep(5 * time.Second)
			continue
		}

		meshNodeID = utils.StripAll(pMesh[0])
		//a.Logger.Debugln("Node id:", meshNodeID)
		if strings.Contains(strings.ToLower(meshNodeID), "not defined") {
			//a.Logger.Errorln(meshNodeID)
			time.Sleep(5 * time.Second)
			continue
		}

		meshSuccess = true
	}

	return meshNodeID, nil
}
