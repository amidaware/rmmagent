package tactical

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/amidaware/rmmagent/agent/patching"
	"github.com/amidaware/rmmagent/agent/services"
	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/tactical/config"
	"github.com/amidaware/rmmagent/agent/tactical/rpc"
	"github.com/amidaware/rmmagent/agent/tasks"
	"github.com/amidaware/rmmagent/agent/utils"
	rmm "github.com/amidaware/rmmagent/shared"
	"github.com/go-resty/resty/v2"
	"github.com/kardianos/service"
	trmm "github.com/wh1te909/trmm-shared"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func UninstallCleanup() {
	registry.DeleteKey(registry.LOCAL_MACHINE, `SOFTWARE\TacticalRMM`)
	patching.PatchMgmnt(false)
	CleanupAgentUpdates()
	tasks.CleanupSchedTasks()
}

func AgentUpdate(url, inno, version string) {
	time.Sleep(time.Duration(utils.RandRange(1, 15)) * time.Second)
	system.KillHungUpdates()
	CleanupAgentUpdates()
	updater := filepath.Join(system.GetProgramDirectory(), inno)
	//a.Logger.Infof("Agent updating from %s to %s", a.Version, version)
	//a.Logger.Infoln("Downloading agent update from", url)

	config := config.NewAgentConfig()
	rClient := resty.New()
	rClient.SetCloseConnection(true)
	rClient.SetTimeout(15 * time.Minute)
	rClient.SetDebug(rmm.DEBUG)
	if len(config.Proxy) > 0 {
		rClient.SetProxy(config.Proxy)
	}

	r, err := rClient.R().SetOutput(updater).Get(url)
	if err != nil {
		//a.Logger.Errorln(err)
		system.CMD("net", []string{"start", services.WinSvcName}, 10, false)
		return
	}

	if r.IsError() {
		//a.Logger.Errorln("Download failed with status code", r.StatusCode())
		system.CMD("net", []string{"start", services.WinSvcName}, 10, false)
		return
	}

	dir, err := ioutil.TempDir("", "tacticalrmm")
	if err != nil {
		//a.Logger.Errorln("Agentupdate create tempdir:", err)
		system.CMD("net", []string{"start", services.WinSvcName}, 10, false)
		return
	}

	innoLogFile := filepath.Join(dir, "tacticalrmm.txt")
	args := []string{"/C", updater, "/VERYSILENT", fmt.Sprintf("/LOG=%s", innoLogFile)}
	cmd := exec.Command("cmd.exe", args...)
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}

	cmd.Start()
	time.Sleep(1 * time.Second)
}

func CleanupAgentUpdates() {
	pd := filepath.Join(os.Getenv("ProgramFiles"), system.ProgFilesName)
	cderr := os.Chdir(pd)
	if cderr != nil {
		//a.Logger.Errorln(cderr)
		return
	}

	files, err := filepath.Glob("winagent-v*.exe")
	if err == nil {
		for _, f := range files {
			os.Remove(f)
		}
	}

	cderr = os.Chdir(os.Getenv("TMP"))
	if cderr != nil {
		//a.Logger.Errorln(cderr)
		return
	}

	folders, err := filepath.Glob("tacticalrmm*")
	if err == nil {
		for _, f := range folders {
			os.RemoveAll(f)
		}
	}
}

func AgentUninstall(code string) {
	system.KillHungUpdates()
	tacUninst := filepath.Join(system.GetProgramDirectory(), GetUninstallExe())
	args := []string{"/C", tacUninst, "/VERYSILENT"}
	cmd := exec.Command("cmd.exe", args...)
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	cmd.Start()
}

func GetUninstallExe() string {
	cderr := os.Chdir(system.GetProgramDirectory())
	if cderr == nil {
		files, err := filepath.Glob("unins*.exe")
		if err == nil {
			for _, f := range files {
				if strings.Contains(f, "001") {
					return f
				}
			}
		}
	}

	return "unins000.exe"
}

// RunMigrations cleans up unused stuff from older agents
func RunMigrations() {
	for _, i := range []string{"nssm.exe", "nssm-x86.exe"} {
		nssm := filepath.Join(system.GetProgramDirectory(), i)
		if trmm.FileExists(nssm) {
			os.Remove(nssm)
		}
	}
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

func Start(_ service.Service) error {
	go rpc.RunRPC(NewAgentConfig())
	return nil
}

func GetPython(force bool) {
	if trmm.FileExists(system.GetPythonBin()) && !force {
		return
	}

	var archZip string
	var folder string
	switch runtime.GOARCH {
	case "amd64":
		archZip = "py38-x64.zip"
		folder = "py38-x64"
	case "386":
		archZip = "py38-x32.zip"
		folder = "py38-x32"
	}
	pyFolder := filepath.Join(system.GetProgramDirectory(), folder)
	pyZip := filepath.Join(system.GetProgramDirectory(), archZip)
	//a.Logger.Debugln(pyZip)
	//a.Logger.Debugln(a.PyBin)
	defer os.Remove(pyZip)

	if force {
		os.RemoveAll(pyFolder)
	}

	config := NewAgentConfig()
	rClient := resty.New()
	rClient.SetTimeout(20 * time.Minute)
	rClient.SetRetryCount(10)
	rClient.SetRetryWaitTime(1 * time.Minute)
	rClient.SetRetryMaxWaitTime(15 * time.Minute)
	if len(config.Proxy) > 0 {
		rClient.SetProxy(config.Proxy)
	}

	url := fmt.Sprintf("https://github.com/amidaware/rmmagent/releases/download/v2.0.0/%s", archZip)
	//a.Logger.Debugln(url)
	r, err := rClient.R().SetOutput(pyZip).Get(url)
	if err != nil {
		//a.Logger.Errorln("Unable to download py3.zip from github.", err)
		return
	}
	if r.IsError() {
		//a.Logger.Errorln("Unable to download py3.zip from github. Status code", r.StatusCode())
		return
	}

	err = utils.Unzip(pyZip, system.GetProgramDirectory())
	if err != nil {
		//a.Logger.Errorln(err)
	}
}
