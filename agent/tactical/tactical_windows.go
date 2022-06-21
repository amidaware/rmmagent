package tactical

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/amidaware/rmmagent/agent/patching"
	"github.com/amidaware/rmmagent/agent/services"
	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/tactical/config"
	"github.com/amidaware/rmmagent/agent/tasks"
	"github.com/amidaware/rmmagent/agent/utils"
	rmm "github.com/amidaware/rmmagent/shared"
	"github.com/go-resty/resty/v2"
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
		system.CMD("net", []string{"start", services.WinSvcName}, 10, false)
		return
	}

	if r.IsError() {
		system.CMD("net", []string{"start", services.WinSvcName}, 10, false)
		return
	}

	dir, err := ioutil.TempDir("", "tacticalrmm")
	if err != nil {
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
