//go:build !windows
// +build !windows

/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	rmm "github.com/amidaware/rmmagent/shared"
	ps "github.com/elastic/go-sysinfo"
	"github.com/go-resty/resty/v2"
	"github.com/jaypipes/ghw"
	"github.com/kardianos/service"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	psHost "github.com/shirou/gopsutil/v3/host"
	"github.com/spf13/viper"
	trmm "github.com/wh1te909/trmm-shared"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func ShowStatus(version string) {
	fmt.Println(version)
}

func (a *Agent) GetDisks() []trmm.Disk {
	ret := make([]trmm.Disk, 0)
	partitions, err := disk.Partitions(false)
	if err != nil {
		a.Logger.Debugln(err)
		return ret
	}

	for _, p := range partitions {
		if strings.Contains(p.Device, "dev/loop") || strings.Contains(p.Device, "devfs") {
			continue
		}
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			a.Logger.Debugln(err)
			continue
		}

		d := trmm.Disk{
			Device:  p.Device,
			Fstype:  p.Fstype,
			Total:   ByteCountSI(usage.Total),
			Used:    ByteCountSI(usage.Used),
			Free:    ByteCountSI(usage.Free),
			Percent: int(usage.UsedPercent),
		}
		ret = append(ret, d)

	}
	return ret
}

func (a *Agent) SystemRebootRequired() (bool, error) {
	// deb
	paths := [2]string{"/var/run/reboot-required", "/run/reboot-required"}
	for _, p := range paths {
		if trmm.FileExists(p) {
			return true, nil
		}
	}
	// rhel
	bins := [2]string{"/usr/bin/needs-restarting", "/bin/needs-restarting"}
	for _, bin := range bins {
		if trmm.FileExists(bin) {
			opts := a.NewCMDOpts()
			// https://man7.org/linux/man-pages/man1/needs-restarting.1.html
			// -r Only report whether a full reboot is required (exit code 1) or not (exit code 0).
			opts.Command = fmt.Sprintf("%s -r", bin)
			out := a.CmdV2(opts)

			if out.Status.Error != nil {
				a.Logger.Debugln("SystemRebootRequired(): ", out.Status.Error.Error())
				continue
			}

			if out.Status.Exit == 1 {
				return true, nil
			}

			return false, nil
		}
	}
	return false, nil
}

func (a *Agent) LoggedOnUser() string {
	var ret string
	users, err := psHost.Users()
	if err != nil {
		return ret
	}

	// return the first logged in user
	for _, user := range users {
		if user.User != "" {
			ret = user.User
			break
		}
	}
	return ret
}

func (a *Agent) osString() string {
	h, err := psHost.Info()
	if err != nil {
		return "error getting host info"
	}
	plat := cases.Title(language.AmericanEnglish).String(h.Platform)
	return fmt.Sprintf("%s %s %s %s", plat, h.PlatformVersion, h.KernelArch, h.KernelVersion)
}

func NewAgentConfig() *rmm.AgentConfig {
	viper.SetConfigName("tacticalagent")
	viper.SetConfigType("json")
	viper.AddConfigPath("/etc/")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()

	if err != nil {
		return &rmm.AgentConfig{}
	}

	agentpk := viper.GetString("agentpk")
	pk, _ := strconv.Atoi(agentpk)

	ret := &rmm.AgentConfig{
		BaseURL:          viper.GetString("baseurl"),
		AgentID:          viper.GetString("agentid"),
		APIURL:           viper.GetString("apiurl"),
		Token:            viper.GetString("token"),
		AgentPK:          agentpk,
		PK:               pk,
		Cert:             viper.GetString("cert"),
		Proxy:            viper.GetString("proxy"),
		CustomMeshDir:    viper.GetString("meshdir"),
		NatsProxyPath:    viper.GetString("natsproxypath"),
		NatsProxyPort:    viper.GetString("natsproxyport"),
		NatsStandardPort: viper.GetString("natsstandardport"),
		NatsPingInterval: viper.GetInt("natspinginterval"),
		Insecure:         viper.GetString("insecure"),
	}
	return ret
}

func (a *Agent) RunScript(code string, shell string, args []string, timeout int, runasuser bool, envVars []string, nushellEnableConfig bool, denoDefaultPermissions string) (stdout, stderr string, exitcode int, e error) {
	code = removeWinNewLines(code)
	content := []byte(code)

	f, err := createNixTmpFile(shell)
	if err != nil {
		a.Logger.Errorln("RunScript createNixTmpFile()", err)
		return "", err.Error(), 85, err
	}
	defer os.Remove(f.Name())

	if _, err := f.Write(content); err != nil {
		a.Logger.Errorln(err)
		return "", err.Error(), 85, err
	}

	if err := f.Close(); err != nil {
		a.Logger.Errorln(err)
		return "", err.Error(), 85, err
	}

	if err := os.Chmod(f.Name(), 0770); err != nil {
		a.Logger.Errorln(err)
		return "", err.Error(), 85, err
	}

	opts := a.NewCMDOpts()
	opts.IsScript = true
	switch shell {
	case "nushell":
		var nushellArgs []string
		if nushellEnableConfig {
			nushellArgs = []string{
				"--config",
				filepath.Join(nixAgentEtcDir, "nushell", "config.nu"),
				"--env-config",
				filepath.Join(nixAgentEtcDir, "nushell", "env.nu"),
			}
		} else {
			nushellArgs = []string{"--no-config-file"}
		}
		opts.Shell = a.NuBin
		opts.Args = nushellArgs
		opts.Args = append(opts.Args, f.Name())
		opts.Args = append(opts.Args, args...)
		if !trmm.FileExists(a.NuBin) {
			a.Logger.Errorln("RunScript(): Executable does not exist. Install Nu and try again:", a.NuBin)
			err := errors.New("File Not Found: " + a.NuBin)
			return "", err.Error(), 85, err
		}

	case "deno":
		opts.Shell = a.DenoBin
		opts.Args = []string{
			"run",
			"--no-prompt",
		}
		if !trmm.FileExists(a.DenoBin) {
			a.Logger.Errorln("RunScript(): Executable does not exist. Install deno and try again:", a.DenoBin)
			err := errors.New("File Not Found: " + a.DenoBin)
			return "", err.Error(), 85, err
		}

		// Search the environment variables for DENO_PERMISSIONS and use that to set permissions for the script.
		// https://docs.deno.com/runtime/manual/basics/permissions#permissions-list
		// DENO_PERMISSIONS is not an official environment variable.
		// https://docs.deno.com/runtime/manual/basics/env_variables
		// DENO_DEFAULT_PERMISSIONS is used if not found in the environment variables.
		found := false
		for i, v := range envVars {
			if strings.HasPrefix(v, "DENO_PERMISSIONS=") {
				permissions := strings.Split(v, "=")[1]
				opts.Args = append(opts.Args, strings.Split(permissions, " ")...)
				// Remove the DENO_PERMISSIONS variable from the environment variables slice.
				// It's possible more variables may exist with the same prefix.
				envVars = append(envVars[:i], envVars[i+1:]...)
				found = true
				break
			}
		}
		if !found && denoDefaultPermissions != "" {
			opts.Args = append(opts.Args, strings.Split(denoDefaultPermissions, " ")...)
		}

		// Can't append a variadic slice after a string arg.
		// https://pkg.go.dev/builtin#append
		opts.Args = append(opts.Args, f.Name())
		opts.Args = append(opts.Args, args...)

	default:
		opts.Shell = f.Name()
		opts.Args = args
	}

	opts.EnvVars = envVars
	opts.Timeout = time.Duration(timeout)
	a.Logger.Debugln("RunScript():", opts.Shell, opts.Args)
	out := a.CmdV2(opts)
	retError := ""
	if out.Status.Error != nil {
		retError += CleanString(out.Status.Error.Error())
		retError += "\n"
	}
	if len(out.Stderr) > 0 {
		retError += out.Stderr
	}
	return out.Stdout, retError, out.Status.Exit, nil
}

func SetDetached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func (a *Agent) seEnforcing() bool {
	opts := a.NewCMDOpts()
	opts.Command = "getenforce"
	out := a.CmdV2(opts)
	return out.Status.Exit == 0 && strings.Contains(out.Stdout, "Enforcing")
}

func (a *Agent) AgentUpdate(url, inno, version string) error {

	self, err := os.Executable()
	if err != nil {
		a.Logger.Errorln("AgentUpdate() os.Executable():", err)
		return err
	}

	// more reliable method to get current working directory than os.Getwd()
	cwd := filepath.Dir(self)
	// create a tmpfile in same location as current binary
	// avoids issues with /tmp dir and other fs mount issues
	f, err := os.CreateTemp(cwd, "trmm")
	if err != nil {
		a.Logger.Errorln("AgentUpdate() os.CreateTemp:", err)
		return err
	}
	defer os.Remove(f.Name())

	a.Logger.Infof("Agent updating from %s to %s", a.Version, version)
	a.Logger.Debugln("Downloading agent update from", url)

	rClient := resty.New()
	rClient.SetCloseConnection(true)
	rClient.SetTimeout(15 * time.Minute)
	rClient.SetDebug(a.Debug)
	if len(a.Proxy) > 0 {
		rClient.SetProxy(a.Proxy)
	}
	if a.Insecure {
		insecureConf := &tls.Config{
			InsecureSkipVerify: true,
		}
		rClient.SetTLSClientConfig(insecureConf)
	}

	r, err := rClient.R().SetOutput(f.Name()).Get(url)
	if err != nil {
		a.Logger.Errorln("AgentUpdate() download:", err)
		f.Close()
		return err
	}
	if r.IsError() {
		a.Logger.Errorln("AgentUpdate() status code:", r.StatusCode())
		f.Close()
		return errors.New("err")
	}

	f.Close()
	os.Chmod(f.Name(), 0755)
	err = os.Rename(f.Name(), self)
	if err != nil {
		a.Logger.Errorln("AgentUpdate() os.Rename():", err)
		return err
	}

	if runtime.GOOS == "linux" && a.seEnforcing() {
		se := a.NewCMDOpts()
		se.Command = fmt.Sprintf("restorecon -rv %s", self)
		out := a.CmdV2(se)
		a.Logger.Debugf("%+v\n", out)
	}

	opts := a.NewCMDOpts()
	opts.Detached = true
	switch runtime.GOOS {
	case "linux":
		opts.Command = "systemctl restart tacticalagent.service"
	case "darwin":
		opts.Command = "launchctl kickstart -k system/tacticalagent"
	default:
		return nil
	}

	a.CmdV2(opts)
	return nil
}

func (a *Agent) AgentUninstall(code string) {
	f, err := createNixTmpFile()
	if err != nil {
		a.Logger.Errorln("AgentUninstall createNixTmpFile():", err)
		return
	}

	f.Write([]byte(code))
	f.Close()
	os.Chmod(f.Name(), 0770)

	opts := a.NewCMDOpts()
	opts.IsScript = true
	opts.Shell = f.Name()
	if runtime.GOOS == "linux" {
		opts.Args = []string{"uninstall"}
	}
	opts.Detached = true
	a.CmdV2(opts)
}

func (a *Agent) NixMeshNodeID() string {
	var meshNodeID string
	meshSuccess := false
	a.Logger.Debugln("Getting mesh node id")

	if !trmm.FileExists(a.MeshSystemEXE) {
		a.Logger.Debugln(a.MeshSystemEXE, "does not exist. Skipping.")
		return ""
	}

	opts := a.NewCMDOpts()
	opts.IsExecutable = true
	opts.Shell = a.MeshSystemEXE
	opts.Command = "-nodeid"

	for !meshSuccess {
		out := a.CmdV2(opts)
		meshNodeID = out.Stdout
		a.Logger.Debugln("Stdout:", out.Stdout)
		a.Logger.Debugln("Stderr:", out.Stderr)
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

func (a *Agent) getMeshNodeID() (string, error) {
	return a.NixMeshNodeID(), nil
}

func (a *Agent) RecoverMesh() {
	a.Logger.Infoln("Attempting mesh recovery")
	opts := a.NewCMDOpts()
	def := "systemctl restart meshagent.service"
	switch runtime.GOOS {
	case "linux":
		opts.Command = def
	case "darwin":
		opts.Command = "launchctl kickstart -k system/meshagent"
	default:
		opts.Command = def
	}
	a.CmdV2(opts)
	a.SyncMeshNodeID()
}

func (a *Agent) GetWMIInfo() map[string]interface{} {
	wmiInfo := make(map[string]interface{})
	ips := make([]string, 0)
	disks := make([]string, 0)
	cpus := make([]string, 0)
	gpus := make([]string, 0)

	// local ips
	host, err := ps.Host()
	if err != nil {
		a.Logger.Errorln("GetWMIInfo() ps.Host()", err)
	} else {
		for _, ip := range host.Info().IPs {
			if strings.Contains(ip, "127.0.") || strings.Contains(ip, "::1/128") {
				continue
			}
			ips = append(ips, ip)
		}
	}
	wmiInfo["local_ips"] = ips

	// disks
	block, err := ghw.Block(ghw.WithDisableWarnings())
	ignore := []string{"ram", "loop"}
	if err != nil {
		a.Logger.Errorln("ghw.Block()", err)
	} else {
		for _, disk := range block.Disks {
			if disk.IsRemovable || contains(disk.Name, ignore) {
				continue
			}
			ret := fmt.Sprintf("%s %s %s %s %s %s", disk.Vendor, disk.Model, disk.StorageController, disk.DriveType, disk.Name, ByteCountSI(disk.SizeBytes))
			ret = strings.TrimSpace(strings.ReplaceAll(ret, "unknown", ""))
			disks = append(disks, ret)
		}
	}
	wmiInfo["disks"] = disks

	// cpus
	cpuInfo, err := cpu.Info()
	if err != nil {
		a.Logger.Errorln("cpu.Info()", err)
	} else {
		if len(cpuInfo) > 0 {
			if cpuInfo[0].ModelName != "" {
				cpus = append(cpus, cpuInfo[0].ModelName)
			}
		}
	}
	wmiInfo["cpus"] = cpus

	// make/model
	wmiInfo["make_model"] = ""
	chassis, err := ghw.Chassis(ghw.WithDisableWarnings())
	if err != nil {
		a.Logger.Debugln("ghw.Chassis()", err)
	} else {
		if chassis.Vendor != "" || chassis.Version != "" {
			wmiInfo["make_model"] = fmt.Sprintf("%s %s", chassis.Vendor, chassis.Version)
		}
	}

	if runtime.GOOS == "darwin" {
		opts := a.NewCMDOpts()
		opts.Command = "sysctl hw.model"
		out := a.CmdV2(opts)
		wmiInfo["make_model"] = strings.ReplaceAll(out.Stdout, "hw.model: ", "")
	}

	// gfx cards

	gpu, err := ghw.GPU(ghw.WithDisableWarnings())
	if err != nil {
		a.Logger.Debugln("ghw.GPU()", err)
	} else {
		for _, i := range gpu.GraphicsCards {
			if i.DeviceInfo != nil {
				ret := fmt.Sprintf("%s %s", i.DeviceInfo.Vendor.Name, i.DeviceInfo.Product.Name)
				gpus = append(gpus, ret)
			}

		}
	}
	wmiInfo["gpus"] = gpus

	// temp hack for ARM cpu/make/model if rasp pi
	var makeModel string
	if strings.Contains(runtime.GOARCH, "arm") {
		file, _ := os.Open("/proc/cpuinfo")
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if strings.Contains(strings.ToLower(scanner.Text()), "raspberry") {
				model := strings.Split(scanner.Text(), ":")
				if len(model) == 2 {
					makeModel = strings.TrimSpace(model[1])
					break
				}
			}
		}
	}

	switch runtime.GOOS {
	case "linux":
		baseboard, err := ghw.Baseboard()
		if err != nil {
			a.Logger.Debugln("ghw.Baseboard()", err)
			wmiInfo["serialnumber"] = "n/a"
		} else {
			wmiInfo["serialnumber"] = baseboard.SerialNumber
		}
	case "darwin":
		opts := a.NewCMDOpts()
		serialCmd := `ioreg -l | grep IOPlatformSerialNumber | grep -o '"IOPlatformSerialNumber" = "[^"]*"' | awk -F'"' '{print $4}'`
		opts.Command = serialCmd
		out := a.CmdV2(opts)
		if out.Status.Error != nil {
			a.Logger.Debugln("ioreg get serial number: ", out.Status.Error.Error())
			wmiInfo["serialnumber"] = "n/a"
		} else {
			wmiInfo["serialnumber"] = removeNewlines(out.Stdout)
		}
	default:
		wmiInfo["serialnumber"] = "n/a"
	}

	if len(cpus) == 0 {
		wmiInfo["cpus"] = []string{makeModel}
	}
	if makeModel != "" && (wmiInfo["make_model"] == "" || wmiInfo["make_model"] == "unknown unknown") {
		wmiInfo["make_model"] = makeModel
	}
	if len(gpus) == 1 && gpus[0] == "unknown unknown" {
		wmiInfo["gpus"] = ""
	}

	return wmiInfo
}

// InstallNushell will download nushell from GitHub and install (copy) it to nixAgentBinDir
func (a *Agent) InstallNushell(force bool) {
	sleepDelay := randRange(1, 10)
	a.Logger.Debugf("InstallNushell() sleeping for %v seconds", sleepDelay)
	time.Sleep(time.Duration(sleepDelay) * time.Second)

	conf := a.GetAgentCheckInConfig(a.GetCheckInConfFromAPI())
	if !conf.InstallNushell {
		return
	}

	if trmm.FileExists(a.NuBin) {
		if force {
			a.Logger.Debugln(a.NuBin, "InstallNushell(): Forced install. Removing nu binary.")
			err := os.Remove(a.NuBin)
			if err != nil {
				a.Logger.Errorln("InstallNushell(): Error removing nu binary:", err)
				return
			}
		} else {
			return
		}
	}

	if !trmm.FileExists(nixAgentBinDir) {
		err := os.MkdirAll(nixAgentBinDir, 0755)
		if err != nil {
			a.Logger.Errorln("InstallNushell(): Error creating nixAgentBinDir:", err)
			return
		}
	}

	if conf.NushellEnableConfig {
		// Create 0-byte config files for Nushell
		nushellPath := filepath.Join(nixAgentEtcDir, "nushell")
		nushellConfig := filepath.Join(nushellPath, "config.nu")
		nushellEnv := filepath.Join(nushellPath, "env.nu")
		if !trmm.FileExists(nushellPath) {
			err := os.MkdirAll(nushellPath, 0755)
			if err != nil {
				a.Logger.Errorln("InstallNushell(): Error creating nixAgentEtcDir/nushell:", err)
				return
			}
		}

		if !trmm.FileExists(nushellConfig) {
			_, err := os.Create(nushellConfig)
			if err != nil {
				a.Logger.Errorln("InstallNushell(): Error creating nushell config.nu:", err)
				return
			}
			err = os.Chmod(nushellConfig, 0744)
			if err != nil {
				a.Logger.Errorln("InstallNushell(): Error changing permissions for nushell config.nu:", err)
				return
			}
		}
		if !trmm.FileExists(nushellEnv) {
			_, err := os.Create(nushellEnv)
			if err != nil {
				a.Logger.Errorln("InstallNushell(): Error creating nushell env.nu:", err)
				return
			}
			err = os.Chmod(nushellEnv, 0744)
			if err != nil {
				a.Logger.Errorln("InstallNushell(): Error changing permissions for nushell env.nu:", err)
				return
			}
		}
	}

	var (
		assetName    string
		url          string
		targzDirName string
	)

	if conf.InstallNushellUrl != "" {
		url = conf.InstallNushellUrl
		url = strings.ReplaceAll(url, "{OS}", runtime.GOOS)
		url = strings.ReplaceAll(url, "{ARCH}", runtime.GOARCH)
		url = strings.ReplaceAll(url, "{VERSION}", conf.InstallNushellVersion)
	} else {
		switch runtime.GOOS {
		case "darwin":
			switch runtime.GOARCH {
			case "arm64":
				// https://github.com/nushell/nushell/releases/download/0.87.0/nu-0.87.0-aarch64-darwin-full.tar.gz
				assetName = fmt.Sprintf("nu-%s-aarch64-darwin-full.tar.gz", conf.InstallNushellVersion)
			default:
				a.Logger.Debugln("InstallNushell(): Unsupported architecture and OS:", runtime.GOARCH, runtime.GOOS)
				return
			}
		case "linux":
			switch runtime.GOARCH {
			case "amd64":
				// https://github.com/nushell/nushell/releases/download/0.87.0/nu-0.87.0-x86_64-linux-musl-full.tar.gz
				assetName = fmt.Sprintf("nu-%s-x86_64-linux-musl-full.tar.gz", conf.InstallNushellVersion)
			case "arm64":
				// https://github.com/nushell/nushell/releases/download/0.87.0/nu-0.87.0-aarch64-linux-gnu-full.tar.gz
				assetName = fmt.Sprintf("nu-%s-aarch64-linux-gnu-full.tar.gz", conf.InstallNushellVersion)
			default:
				a.Logger.Debugln("InstallNushell(): Unsupported architecture and OS:", runtime.GOARCH, runtime.GOOS)
				return
			}
		default:
			a.Logger.Debugln("InstallNushell(): Unsupported OS:", runtime.GOOS)
			return
		}
		url = fmt.Sprintf("https://github.com/nushell/nushell/releases/download/%s/%s", conf.InstallNushellVersion, assetName)
	}
	a.Logger.Debugln("InstallNushell(): Nu download url:", url)

	tmpDir, err := os.MkdirTemp("", "nutemp")
	if err != nil {
		a.Logger.Errorln("InstallNushell(): Error creating nushell temp directory:", err)
		return
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			a.Logger.Errorln("InstallNushell(): Error removing nushell temp directory:", err)
		}
	}(tmpDir)

	tmpAssetName := filepath.Join(tmpDir, assetName)
	a.Logger.Debugln("InstallNushell(): tmpAssetName:", tmpAssetName)

	rClient := resty.New()
	rClient.SetTimeout(20 * time.Minute)
	rClient.SetRetryCount(10)
	rClient.SetRetryWaitTime(1 * time.Minute)
	rClient.SetRetryMaxWaitTime(15 * time.Minute)
	if len(a.Proxy) > 0 {
		rClient.SetProxy(a.Proxy)
	}

	r, err := rClient.R().SetOutput(tmpAssetName).Get(url)
	if err != nil {
		a.Logger.Errorln("InstallNushell(): Unable to download nu:", err)
		return
	}
	if r.IsError() {
		a.Logger.Errorln("InstallNushell(): Unable to download nu. Status code:", r.StatusCode())
		return
	}

	if conf.InstallNushellUrl != "" {
		// InstallNushellUrl is not compressed.
		err = copyFile(filepath.Join(tmpDir, tmpAssetName), a.NuBin)
		if err != nil {
			a.Logger.Errorln("InstallNushell(): Failed to copy nu file to install dir:", err)
			return
		}
	} else {
		// GitHub asset is tar.gz compressed.
		targzDirName, err = a.ExtractTarGz(tmpAssetName, tmpDir)
		if err != nil {
			a.Logger.Errorln("InstallNushell(): Failed to extract downloaded tar.gz file:", err)
			return
		}

		err = copyFile(filepath.Join(tmpDir, targzDirName, "nu"), a.NuBin)
		if err != nil {
			a.Logger.Errorln("InstallNushell(): Failed to copy nu file to install dir:", err)
			return
		}
	}

	err = os.Chmod(a.NuBin, 0755)
	if err != nil {
		a.Logger.Errorln("InstallNushell(): Failed to chmod nu binary:", err)
		return
	}

}

// InstallDeno will download deno from GitHub and install (copy) it to nixAgentBinDir
func (a *Agent) InstallDeno(force bool) {
	sleepDelay := randRange(1, 10)
	a.Logger.Debugf("InstallDeno() sleeping for %v seconds", sleepDelay)
	time.Sleep(time.Duration(sleepDelay) * time.Second)

	conf := a.GetAgentCheckInConfig(a.GetCheckInConfFromAPI())
	if !conf.InstallDeno {
		return
	}

	if trmm.FileExists(a.DenoBin) {
		if force {
			a.Logger.Debugln(a.NuBin, "InstallDeno(): Forced install. Removing deno binary.")
			err := os.Remove(a.DenoBin)
			if err != nil {
				a.Logger.Errorln("InstallDeno(): Error removing deno binary:", err)
				return
			}
		} else {
			return
		}
	}

	if !trmm.FileExists(nixAgentBinDir) {
		err := os.MkdirAll(nixAgentBinDir, 0755)
		if err != nil {
			a.Logger.Errorln("InstallDeno(): Error creating nixAgentBinDir:", err)
			return
		}
	}

	var (
		assetName string
		url       string
	)

	if conf.InstallDenoUrl != "" {
		url = conf.InstallDenoUrl
		url = strings.ReplaceAll(url, "{OS}", runtime.GOOS)
		url = strings.ReplaceAll(url, "{ARCH}", runtime.GOARCH)
		url = strings.ReplaceAll(url, "{VERSION}", conf.InstallDenoVersion)
	} else {
		switch runtime.GOOS {
		case "darwin":
			switch runtime.GOARCH {
			case "arm64":
				// https://github.com/denoland/deno/releases/download/v1.38.2/deno-aarch64-apple-darwin.zip
				assetName = "deno-aarch64-apple-darwin.zip"
			case "amd64":
				// https://github.com/denoland/deno/releases/download/v1.38.2/deno-x86_64-apple-darwin.zip
				assetName = "deno-x86_64-apple-darwin.zip"
			default:
				a.Logger.Debugln("InstallDeno(): Unsupported architecture and OS:", runtime.GOARCH, runtime.GOOS)
				return
			}
		case "linux":
			switch runtime.GOARCH {
			case "amd64":
				// https://github.com/denoland/deno/releases/download/v1.38.2/deno-x86_64-unknown-linux-gnu.zip
				assetName = "deno-x86_64-unknown-linux-gnu.zip"
			default:
				a.Logger.Debugln("InstallDeno(): Unsupported architecture and OS:", runtime.GOARCH, runtime.GOOS)
				return
			}
		default:
			a.Logger.Debugln("InstallDeno(): Unsupported OS:", runtime.GOOS)
			return
		}
		url = fmt.Sprintf("https://github.com/denoland/deno/releases/download/%s/%s", conf.InstallDenoVersion, assetName)
	}
	a.Logger.Debugln("InstallDeno(): Deno download url:", url)

	tmpDir, err := os.MkdirTemp("", "denotemp")
	if err != nil {
		a.Logger.Errorln("InstallDeno(): Error creating deno temp directory:", err)
		return
	}
	defer func(path string) {
		err := os.RemoveAll(path)
		if err != nil {
			a.Logger.Errorln("InstallDeno(): Error removing deno temp directory:", err)
		}
	}(tmpDir)

	tmpAssetName := filepath.Join(tmpDir, assetName)
	a.Logger.Debugln("InstallDeno(): tmpAssetName:", tmpAssetName)

	rClient := resty.New()
	rClient.SetTimeout(20 * time.Minute)
	rClient.SetRetryCount(10)
	rClient.SetRetryWaitTime(1 * time.Minute)
	rClient.SetRetryMaxWaitTime(15 * time.Minute)
	if len(a.Proxy) > 0 {
		rClient.SetProxy(a.Proxy)
	}

	r, err := rClient.R().SetOutput(tmpAssetName).Get(url)
	if err != nil {
		a.Logger.Errorln("InstallDeno(): Unable to download deno:", err)
		return
	}
	if r.IsError() {
		a.Logger.Errorln("InstallDeno(): Unable to download deno. Status code:", r.StatusCode())
		return
	}

	if conf.InstallDenoUrl != "" {
		// InstallDenoUrl is not compressed.
		err = copyFile(filepath.Join(tmpDir, tmpAssetName), a.DenoBin)
		if err != nil {
			a.Logger.Errorln("InstallDeno(): Failed to copy deno file to install dir:", err)
			return
		}
	} else {
		// GitHub asset is zip compressed.
		err = Unzip(tmpAssetName, tmpDir)
		if err != nil {
			a.Logger.Errorln("InstallDeno(): Failed to unzip downloaded zip file:", err)
			return
		}

		err = copyFile(filepath.Join(tmpDir, "deno"), a.DenoBin)
		if err != nil {
			a.Logger.Errorln("InstallDeno(): Failed to copy deno file to install dir:", err)
			return
		}
	}

	err = os.Chmod(a.DenoBin, 0755)
	if err != nil {
		a.Logger.Errorln("InstallDeno(): Failed to chmod deno binary:", err)
		return
	}

}

// GetAgentCheckInConfig will get the agent configuration from the server.
// The Windows agent stores the configuration in the registry. The UNIX agent does not store the config.
// @return AgentCheckInConfig
func (a *Agent) GetAgentCheckInConfig(ret AgentCheckInConfig) AgentCheckInConfig {
	// TODO: Persist the config to disk.
	return ret
}

// windows only below TODO add into stub file
func (a *Agent) PlatVer() (string, error) { return "", nil }

func (a *Agent) SendSoftware() {}

func (a *Agent) UninstallCleanup() {}

func (a *Agent) RunMigrations() {}

func GetServiceStatus(name string) (string, error) { return "", nil }

func (a *Agent) GetPython(force bool) {}

type SchedTask struct{ Name string }

func (a *Agent) PatchMgmnt(enable bool) error { return nil }

func (a *Agent) CreateSchedTask(st SchedTask) (bool, error) { return false, nil }

func DeleteSchedTask(name string) error { return nil }

func ListSchedTasks() []string { return []string{} }

func (a *Agent) GetEventLog(logName string, searchLastDays int) []rmm.EventLogMsg {
	return []rmm.EventLogMsg{}
}

func (a *Agent) GetServiceDetail(name string) trmm.WindowsService { return trmm.WindowsService{} }

func (a *Agent) ControlService(name, action string) rmm.WinSvcResp {
	return rmm.WinSvcResp{Success: false, ErrorMsg: "/na"}
}

func (a *Agent) EditService(name, startupType string) rmm.WinSvcResp {
	return rmm.WinSvcResp{Success: false, ErrorMsg: "/na"}
}

func (a *Agent) GetInstalledSoftware() []trmm.WinSoftwareList { return []trmm.WinSoftwareList{} }

func (a *Agent) ChecksRunning() bool { return false }

func (a *Agent) InstallChoco() {}

func (a *Agent) InstallWithChoco(name string) (string, error) { return "", nil }

func (a *Agent) GetWinUpdates() {}

func (a *Agent) InstallUpdates(guids []string) {}

func (a *Agent) installMesh(meshbin, exe, proxy string) (string, error) {
	return "not implemented", nil
}

func CMDShell(shell string, cmdArgs []string, command string, timeout int, detached bool, runasuser bool) (output [2]string, e error) {
	return [2]string{"", ""}, nil
}

func CMD(exe string, args []string, timeout int, detached bool) (output [2]string, e error) {
	return [2]string{"", ""}, nil
}

func (a *Agent) GetServices() []trmm.WindowsService { return []trmm.WindowsService{} }

func (a *Agent) Start(_ service.Service) error { return nil }

func (a *Agent) Stop(_ service.Service) error { return nil }

func (a *Agent) InstallService() error { return nil }
