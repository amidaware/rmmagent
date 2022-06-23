//go:build !windows
// +build !windows

package tactical

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/tactical/mesh"
	"github.com/amidaware/rmmagent/agent/tactical/shared"
	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"
	"github.com/wh1te909/trmm-shared"
)

func GetMeshBinary() string {
	return "/opt/tacticalmesh/meshagent"
}

func NewAgentConfig() *AgentConfig {
	viper.SetConfigName("tacticalagent")
	viper.SetConfigType("json")
	viper.AddConfigPath("/etc/")
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()

	if err != nil {
		return &AgentConfig{}
	}

	agentpk := viper.GetString("agentpk")
	pk, _ := strconv.Atoi(agentpk)

	ret := &AgentConfig{
		BaseURL:       viper.GetString("baseurl"),
		AgentID:       viper.GetString("agentid"),
		APIURL:        viper.GetString("apiurl"),
		Token:         viper.GetString("token"),
		AgentPK:       agentpk,
		PK:            pk,
		Cert:          viper.GetString("cert"),
		Proxy:         viper.GetString("proxy"),
		CustomMeshDir: viper.GetString("meshdir"),
	}

	return ret
}

func AgentUpdate(url string, inno string) bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}

	f, err := utils.CreateTmpFile()
	if err != nil {
		return false
	}
	defer os.Remove(f.Name())

	rClient := resty.New()
	rClient.SetCloseConnection(true)
	rClient.SetTimeout(15 * time.Minute)
	//if shared.DEBUG {
		//rClient.SetDebug(true)
	//}

	config := NewAgentConfig()
	if len(config.Proxy) > 0 {
		rClient.SetProxy(config.Proxy)
	}

	r, err := rClient.R().SetOutput(f.Name()).Get(url)
	if err != nil {
		//a.Logger.Errorln("AgentUpdate() download:", err)
		f.Close()
		return false
	}
	if r.IsError() {
		//a.Logger.Errorln("AgentUpdate() status code:", r.StatusCode())
		f.Close()
		return false
	}

	f.Close()
	os.Chmod(f.Name(), 0755)
	err = os.Rename(f.Name(), self)
	if err != nil {
		//a.Logger.Errorln("AgentUpdate() os.Rename():", err)
		return false
	}

	opts := system.NewCMDOpts()
	opts.Detached = true
	opts.Command = "systemctl restart tacticalagent.service"
	system.CmdV2(opts)
	return true
}

func AgentUninstall(code string) bool {
	f, err := utils.CreateTmpFile()
	if err != nil {
		//a.Logger.Errorln("AgentUninstall createTmpFile():", err)
		return false
	}

	f.Write([]byte(code))
	f.Close()
	os.Chmod(f.Name(), 0770)

	opts := system.NewCMDOpts()
	opts.IsScript = true
	opts.Shell = f.Name()
	opts.Args = []string{"uninstall"}
	opts.Detached = true
	system.CmdV2(opts)

	return true
}

func NixMeshNodeID() string {
	var meshNodeID string
	meshSuccess := false
	//a.Logger.Debugln("Getting mesh node id")

	if !trmm.FileExists(GetMeshBinary()) {
		//a.Logger.Debugln(a.MeshSystemBin, "does not exist. Skipping.")
		return ""
	}

	opts := system.NewCMDOpts()
	opts.IsExecutable = true
	opts.Shell = GetMeshBinary()
	opts.Command = "-nodeid"

	for !meshSuccess {
		out := system.CmdV2(opts)
		meshNodeID = out.Stdout
		//a.Logger.Debugln("Stdout:", out.Stdout)
		//a.Logger.Debugln("Stderr:", out.Stderr)
		if meshNodeID == "" {
			time.Sleep(1 * time.Second)
			continue
		} else if strings.Contains(strings.ToLower(meshNodeID), "graphical version") || strings.Contains(strings.ToLower(meshNodeID), "zenity") {
			time.Sleep(1 * time.Second)
			continue
		}

		meshSuccess = true
	}

	return meshNodeID
}

func GetMeshNodeID() (string, error) {
	return NixMeshNodeID(), nil
}

func RecoverMesh(agentID string) {
	//a.Logger.Infoln("Attempting mesh recovery")
	opts := system.NewCMDOpts()
	opts.Command = "systemctl restart meshagent.service"
	system.CmdV2(opts)
	mesh.SyncMeshNodeID()
}

func UninstallCleanup() {}

func RunMigrations() {}

func GetPython(force bool) {}

func ChecksRunning() bool { return false }

func RunTask(id int) error { return nil }

func installMesh(meshbin, exe, proxy string) (string, error) {
	return "not implemented", nil
}

func SendSoftware() {}

func GetVersion() string {
	version, err := exec.Command(shared.GetProgramBin(), "-version").Output()
	if err != nil {
		return ""
	}

	re := regexp.MustCompile(`Tactical RMM Agent: v([0-9]\.[0-9]\.[0-9])`)
	match := re.FindStringSubmatch(string(version))
	return match[1]
}