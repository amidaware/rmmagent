package install

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/amidaware/rmmagent/agent/patching"
	"github.com/amidaware/rmmagent/agent/services"
	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/tactical"
	"github.com/amidaware/rmmagent/agent/tactical/mesh"
	"github.com/amidaware/rmmagent/agent/tactical/service"
	"github.com/amidaware/rmmagent/agent/tactical/shared"
	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/go-resty/resty/v2"
	"github.com/gonutz/w32/v2"
	ksvc "github.com/kardianos/service"
	"github.com/shirou/gopsutil/host"
	"golang.org/x/sys/windows/registry"
)

const winSvcName = "tacticalrmm"

func Install(i *Installer) error {
	CheckExistingAndRemove(i.Silent)
	i.Headers = map[string]string{
		"content-type":  "application/json",
		"Authorization": fmt.Sprintf("Token %s", i.Token),
	}

	AgentID := GenerateAgentID()
	u, err := url.Parse(i.RMM)
	if err != nil {
		return err
	}

	if u.Scheme != "https" && u.Scheme != "http" {
		return errors.New("Invalid URL (must contain https or http)")
	}

	// will match either ipv4 , or ipv4:port
	var ipPort = regexp.MustCompile(`[0-9]+(?:\.[0-9]+){3}(:[0-9]+)?`)

	// if ipv4:port, strip the port to get ip for salt master
	if ipPort.MatchString(u.Host) && strings.Contains(u.Host, ":") {
		i.SaltMaster = strings.Split(u.Host, ":")[0]
	} else if strings.Contains(u.Host, ":") {
		i.SaltMaster = strings.Split(u.Host, ":")[0]
	} else {
		i.SaltMaster = u.Host
	}

	terr := utils.TestTCP(fmt.Sprintf("%s:4222", i.SaltMaster))
	if terr != nil {
		return fmt.Errorf("ERROR: Either port 4222 TCP is not open on your RMM, or nats.service is not running.\n\n%s", terr.Error())
	}

	baseURL := u.Scheme + "://" + u.Host
	iClient := resty.New()
	iClient.SetCloseConnection(true)
	iClient.SetTimeout(15 * time.Second)
	iClient.SetHeaders(i.Headers)

	// set proxy if applicable
	if len(i.Proxy) > 0 {
		iClient.SetProxy(i.Proxy)
	}

	creds, cerr := iClient.R().Get(fmt.Sprintf("%s/api/v3/installer/", baseURL))
	if cerr != nil {
		return cerr
	}

	if creds.StatusCode() == 401 {
		return errors.New("Installer token has expired. Please generate a new one.")
	}

	verPayload := map[string]string{"version": i.Version}
	iVersion, ierr := iClient.R().SetBody(verPayload).Post(fmt.Sprintf("%s/api/v3/installer/", baseURL))
	if ierr != nil {
		return ierr
	}

	if iVersion.StatusCode() != 200 {
		return errors.New(DjangoStringResp(iVersion.String()))
	}

	rClient := resty.New()
	rClient.SetCloseConnection(true)
	rClient.SetTimeout(i.Timeout * time.Second)
	// set rest knox headers
	rClient.SetHeaders(i.Headers)

	// set local cert if applicable
	if len(i.Cert) > 0 {
		if !utils.FileExists(i.Cert) {
			return fmt.Errorf("%s does not exist", i.Cert)
		}

		rClient.SetRootCertificate(i.Cert)
	}

	if len(i.Proxy) > 0 {
		rClient.SetProxy(i.Proxy)
	}

	var arch string
	switch runtime.GOARCH {
	case "x86_64":
		arch = "64"
	case "x86":
		arch = "32"
	}

	var installerMeshSystemBin string
	if len(i.MeshDir) > 0 {
		installerMeshSystemBin = filepath.Join(i.MeshDir, "MeshAgent.exe")
	} else {
		installerMeshSystemBin = mesh.GetMeshBinLocation()
	}

	var meshNodeID string

	if runtime.GOOS == "windows" && !i.NoMesh {
		meshPath := filepath.Join(shared.GetProgramDirectory(), "meshagent.exe")
		if i.LocalMesh == "" {
			payload := map[string]string{"arch": arch, "plat": runtime.GOOS}
			r, err := rClient.R().SetBody(payload).SetOutput(meshPath).Post(fmt.Sprintf("%s/api/v3/meshexe/", baseURL))
			if err != nil {
			}
			if r.StatusCode() != 200 {
			}
		} else {
			err := copyFile(i.LocalMesh, meshPath)
			if err != nil {
			}
		}

		time.Sleep(1 * time.Second)
		meshNodeID, err = mesh.InstallMesh(meshPath, installerMeshSystemBin, i.Proxy)
		if err != nil {
		}
	}

	if len(i.MeshNodeID) > 0 {
		meshNodeID = i.MeshNodeID
	}

	// add agent
	host, _ := host.Info()
	agentPayload := map[string]interface{}{
		"agent_id":        AgentID,
		"hostname":        host.Hostname,
		"site":            i.SiteID,
		"monitoring_type": i.AgentType,
		"mesh_node_id":    meshNodeID,
		"description":     i.Description,
		"goarch":          runtime.GOARCH,
		"plat":            runtime.GOOS,
	}

	r, err := rClient.R().SetBody(agentPayload).SetResult(&NewAgentResp{}).Post(fmt.Sprintf("%s/api/v3/newagent/", baseURL))
	if err != nil {
	}
	if r.StatusCode() != 200 {
	}

	agentPK := r.Result().(*NewAgentResp).AgentPK
	agentToken := r.Result().(*NewAgentResp).Token
	CreateAgentConfig(baseURL, AgentID, i.SaltMaster, agentToken, strconv.Itoa(agentPK), i.Cert, i.Proxy, i.MeshDir)
	time.Sleep(1 * time.Second)

	time.Sleep(3 * time.Second)

	// check in once
	service.DoNatsCheckIn(i.Version)
	service.SendSoftware()
	utils.CreateTRMMTempDir()
	patching.PatchMgmnt(true)

	svcConf := &ksvc.Config{
		Executable:  shared.GetProgramBin(),
		Name:        winSvcName,
		DisplayName: "TacticalRMM Agent Service",
		Arguments:   []string{"-m", "svc"},
		Description: "TacticalRMM Agent Service",
		Option: ksvc.KeyValue{
			"StartType":              "automatic",
			"OnFailure":              "restart",
			"OnFailureDelayDuration": "5s",
			"OnFailureResetPeriod":   10,
		},
	}

	err = service.InstallService(winSvcName, service.IService{}, svcConf)
	if err != nil {
		return err
	}

	time.Sleep(1 * time.Second)
	out := services.ControlService(winSvcName, "start")
	if !out.Success {
		return errors.New(out.ErrorMsg)
	}

	system.AddDefenderExclusions()
	if i.Power {
		system.DisableSleepHibernate()
	}

	if i.Ping {
		system.EnablePing()
	}

	if i.RDP {
		system.EnableRDP()
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return nil
}

// GenerateAgentID creates and returns a unique agent id
func GenerateAgentID() string {
	rand.Seed(time.Now().UnixNano())
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	b := make([]rune, 40)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// DjangoStringResp removes double quotes from django rest api resp
func DjangoStringResp(resp string) string {
	return strings.Trim(resp, `"`)
}

func CreateAgentConfig(baseurl, agentid, apiurl, token, agentpk, cert, proxy, meshdir string) error {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\TacticalRMM`, registry.ALL_ACCESS)
	if err != nil {
		return err
	}

	defer k.Close()
	err = k.SetStringValue("BaseURL", baseurl)
	if err != nil {
		return err
	}

	err = k.SetStringValue("AgentID", agentid)
	if err != nil {
		return err
	}

	err = k.SetStringValue("ApiURL", apiurl)
	if err != nil {
		return err
	}

	err = k.SetStringValue("Token", token)
	if err != nil {
		return err
	}

	err = k.SetStringValue("AgentPK", agentpk)
	if err != nil {
	}

	if len(cert) > 0 {
		err = k.SetStringValue("Cert", cert)
		if err != nil {
			return err
		}
	}

	if len(proxy) > 0 {
		err = k.SetStringValue("Proxy", proxy)
		if err != nil {
			return err
		}
	}

	if len(meshdir) > 0 {
		err = k.SetStringValue("MeshDir", meshdir)
		if err != nil {
			return err
		}
	}

	return nil
}

func CheckExistingAndRemove(silent bool) {
	hasReg := false
	_, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\TacticalRMM`, registry.ALL_ACCESS)

	if err == nil {
		hasReg = true
	}
	if hasReg {
		tacUninst := filepath.Join(shared.GetProgramDirectory(), tactical.GetUninstallExe())
		tacUninstArgs := [2]string{tacUninst, "/VERYSILENT"}

		window := w32.GetForegroundWindow()
		if !silent && window != 0 {
			var handle w32.HWND
			msg := "Existing installation found\nClick OK to remove, then re-run the installer.\nClick Cancel to abort."
			action := w32.MessageBox(handle, msg, "Tactical RMM", w32.MB_OKCANCEL|w32.MB_ICONWARNING)
			if action == w32.IDOK {
				tactical.AgentUninstall("foo")
			}
		} else {
			fmt.Println("Existing installation found and must be removed before attempting to reinstall.")
			fmt.Println("Run the following command to uninstall, and then re-run this installer.")
			fmt.Printf(`"%s" %s `, tacUninstArgs[0], tacUninstArgs[1])
		}
		os.Exit(0)
	}
}
