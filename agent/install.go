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
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	trmm "github.com/wh1te909/trmm-shared"
)

type Installer struct {
	Headers          map[string]string
	RMM              string
	ClientID         int
	SiteID           int
	Description      string
	AgentType        string
	Power            bool
	RDP              bool
	Ping             bool
	Token            string
	LocalMesh        string
	Cert             string
	Proxy            string
	Timeout          time.Duration
	SaltMaster       string
	Silent           bool
	NoMesh           bool
	MeshDir          string
	MeshNodeID       string
	Insecure         bool
	NatsStandardPort string
}

func (a *Agent) Install(i *Installer) {
	a.checkExistingAndRemove(i.Silent)

	i.Headers = map[string]string{
		"content-type":  "application/json",
		"Authorization": fmt.Sprintf("Token %s", i.Token),
	}
	a.AgentID = GenerateAgentID()
	a.Logger.Debugln("Agent ID:", a.AgentID)

	u, err := url.Parse(i.RMM)
	if err != nil {
		a.installerMsg(err.Error(), "error", i.Silent)
	}

	if u.Scheme != "https" && u.Scheme != "http" {
		a.installerMsg("Invalid URL (must contain https or http)", "error", i.Silent)
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

	a.Logger.Debugln("API:", i.SaltMaster)

	baseURL := u.Scheme + "://" + u.Host
	a.Logger.Debugln("Base URL:", baseURL)

	iClient := resty.New()
	iClient.SetCloseConnection(true)
	iClient.SetTimeout(15 * time.Second)
	iClient.SetDebug(a.Debug)
	iClient.SetHeaders(i.Headers)

	// set proxy if applicable
	if len(i.Proxy) > 0 {
		a.Logger.Infoln("Using proxy:", i.Proxy)
		iClient.SetProxy(i.Proxy)
	}

	insecureConf := &tls.Config{
		InsecureSkipVerify: true,
	}

	if i.Insecure {
		iClient.SetTLSClientConfig(insecureConf)
	}

	creds, cerr := iClient.R().Get(fmt.Sprintf("%s/api/v3/installer/", baseURL))
	if cerr != nil {
		a.installerMsg(cerr.Error(), "error", i.Silent)
	}
	if creds.StatusCode() == 401 {
		a.installerMsg("Installer token has expired. Please generate a new one.", "error", i.Silent)
	}

	verPayload := map[string]string{"version": a.Version}
	iVersion, ierr := iClient.R().SetBody(verPayload).Post(fmt.Sprintf("%s/api/v3/installer/", baseURL))
	if ierr != nil {
		a.installerMsg(ierr.Error(), "error", i.Silent)
	}
	if iVersion.StatusCode() != 200 {
		a.installerMsg(DjangoStringResp(iVersion.String()), "error", i.Silent)
	}

	rClient := resty.New()
	rClient.SetCloseConnection(true)
	rClient.SetTimeout(i.Timeout * time.Second)
	rClient.SetDebug(a.Debug)
	// set rest knox headers
	rClient.SetHeaders(i.Headers)

	// set local cert if applicable
	if len(i.Cert) > 0 {
		if !trmm.FileExists(i.Cert) {
			a.installerMsg(fmt.Sprintf("%s does not exist", i.Cert), "error", i.Silent)
		}
		rClient.SetRootCertificate(i.Cert)
	}

	if len(i.Proxy) > 0 {
		rClient.SetProxy(i.Proxy)
	}

	if i.Insecure {
		rClient.SetTLSClientConfig(insecureConf)
	}

	var installerMeshSystemEXE string
	if len(i.MeshDir) > 0 {
		installerMeshSystemEXE = filepath.Join(i.MeshDir, "MeshAgent.exe")
	} else {
		installerMeshSystemEXE = a.MeshSystemEXE
	}

	var meshNodeID, meshOutput string

	if !i.NoMesh && runtime.GOOS != "linux" {
		var meshArchive, meshExtractDir, meshBinary string

		switch runtime.GOOS {
		case "windows":
			meshOutput = filepath.Join(a.ProgramDir, a.MeshInstaller)
		case "darwin":
			tmp, err := createNixTmpFile()
			if err != nil {
				a.Logger.Fatalln("Failed to create mesh temp file", err)
			}
			meshArchive = tmp.Name() + ".tar.gz"
			meshExtractDir = tmp.Name() + "_extracted_" + generateRandomString(8)
			defer os.Remove(tmp.Name())
			defer os.Remove(meshArchive)
			defer os.RemoveAll(meshExtractDir)
		}

		if runtime.GOOS == "windows" && i.LocalMesh != "" {
			err := copyFile(i.LocalMesh, meshOutput)
			if err != nil {
				a.installerMsg(err.Error(), "error", i.Silent)
			}
		} else if runtime.GOOS == "windows" {
			a.Logger.Infoln("Downloading mesh agent...")
			payload := map[string]string{"goarch": a.GoArch, "plat": a.Platform}
			r, err := rClient.R().SetBody(payload).SetOutput(meshOutput).Post(fmt.Sprintf("%s/api/v3/meshexe/", baseURL))
			if err != nil {
				a.installerMsg(fmt.Sprintf("Failed to download mesh agent: %s", err.Error()), "error", i.Silent)
			}
			if r.StatusCode() != 200 {
				a.installerMsg(fmt.Sprintf("Unable to download the mesh agent from the RMM. %s", r.String()), "error", i.Silent)
			}
		} else if runtime.GOOS == "darwin" {
			// Download tar.gz archive for macOS
			a.Logger.Infoln("Downloading mesh agent archive...")
			payload := map[string]string{"goarch": a.GoArch, "plat": a.Platform}
			r, err := rClient.R().SetBody(payload).SetOutput(meshArchive).Post(fmt.Sprintf("%s/api/v3/meshexe/", baseURL))
			if err != nil {
				a.installerMsg(fmt.Sprintf("Failed to download mesh agent: %s", err.Error()), "error", i.Silent)
			}
			if r.StatusCode() != 200 {
				a.installerMsg(fmt.Sprintf("Unable to download the mesh agent from the RMM. %s", r.String()), "error", i.Silent)
			}

			// Extract archive
			a.Logger.Infoln("Extracting mesh agent archive...")
			os.MkdirAll(meshExtractDir, 0755)
			_, err = a.ExtractTarGz(meshArchive, meshExtractDir)
			if err != nil {
				a.installerMsg(fmt.Sprintf("Failed to extract mesh archive: %s", err.Error()), "error", i.Silent)
			}

			// Verify SHA256SUMS and get actual filenames
			a.Logger.Infoln("Verifying mesh agent integrity...")
			meshFilename, err := verifyMeshFiles(meshExtractDir)
			if err != nil {
				a.installerMsg(fmt.Sprintf("Failed to verify mesh agent: %s", err.Error()), "error", i.Silent)
			}
			a.Logger.Infoln("Mesh agent verification successful")

			// Set path to mesh binary
			meshBinary = filepath.Join(meshExtractDir, meshFilename)

			os.Chmod(meshBinary, 0755)
			meshOutput = meshBinary
		}

		a.Logger.Infoln("Installing mesh agent...")
		a.Logger.Debugln("Mesh agent:", meshOutput)
		time.Sleep(1 * time.Second)

		if runtime.GOOS == "windows" {
			meshNodeID, err = a.installMesh(meshOutput, installerMeshSystemEXE, i.Proxy)
			if err != nil {
				a.installerMsg(fmt.Sprintf("Failed to install mesh agent: %s", err.Error()), "error", i.Silent)
			}
		} else {
			opts := a.NewCMDOpts()
			if runtime.GOOS == "darwin" {
				// For macOS, use --no-embedded and --copy-msh flags with the .msh file
				opts.Command = fmt.Sprintf("%s -install --no-embedded=\"1\" --copy-msh=\"1\" --installPath=%s", meshOutput, nixMeshDir)
			} else {
				opts.Command = fmt.Sprintf("%s -install --installPath=%s", meshOutput, nixMeshDir)
			}
			opts.Timeout = i.Timeout
			out := a.CmdV2(opts)
			if out.Status.Exit != 0 {
				a.Logger.Fatalln("Error installing mesh agent:", out.Stderr)
			}
			fmt.Println(out.Stdout)
		}
	}

	if len(i.MeshNodeID) > 0 {
		meshNodeID = i.MeshNodeID
	}

	a.Logger.Infoln("Adding agent to dashboard")
	// add agent
	type NewAgentResp struct {
		AgentPK int    `json:"pk"`
		Token   string `json:"token"`
	}
	agentPayload := map[string]interface{}{
		"agent_id":        a.AgentID,
		"hostname":        a.Hostname,
		"site":            i.SiteID,
		"monitoring_type": i.AgentType,
		"mesh_node_id":    meshNodeID,
		"description":     i.Description,
		"goarch":          a.GoArch,
		"plat":            a.Platform,
	}

	r, err := rClient.R().SetBody(agentPayload).SetResult(&NewAgentResp{}).Post(fmt.Sprintf("%s/api/v3/newagent/", baseURL))
	if err != nil {
		a.installerMsg(err.Error(), "error", i.Silent)
	}
	if r.StatusCode() != 200 {
		a.installerMsg(r.String(), "error", i.Silent)
	}

	agentPK := r.Result().(*NewAgentResp).AgentPK
	agentToken := r.Result().(*NewAgentResp).Token

	a.Logger.Debugln("Agent token:", agentToken)
	a.Logger.Debugln("Agent PK:", agentPK)

	createAgentConfig(baseURL, a.AgentID, i.SaltMaster, agentToken, strconv.Itoa(agentPK), i.Cert, i.Proxy, i.MeshDir, i.NatsStandardPort, i.Insecure)
	time.Sleep(1 * time.Second)
	// refresh our agent with new values
	a = New(a.Logger, a.Version)
	a.Logger.Debugf("%+v\n", a)

	// set new headers, no longer knox auth...use agent auth
	rClient.SetHeaders(a.Headers)

	time.Sleep(3 * time.Second)
	// check in once
	a.DoNatsCheckIn()

	if runtime.GOOS == "linux" {
		// Used for Nushell and Deno binaries
		os.MkdirAll(nixAgentBinDir, 0755)
	}

	if runtime.GOOS == "darwin" {
		os.MkdirAll(nixAgentBinDir, 0755)
		self, _ := os.Executable()
		copyFile(self, nixAgentBin)
		os.Chmod(nixAgentBin, 0755)
		svc := fmt.Sprintf(`
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple Computer//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
	<dict>
		<key>Label</key>
		<string>%s</string>

		<key>ServiceDescription</key>
        <string>TacticalAgent Service</string>

		<key>ProgramArguments</key>
		<array>
			<string>%s</string>
			<string>-m</string>
			<string>svc</string>
		</array>

		<key>WorkingDirectory</key>
		<string>%s/</string>

		<key>RunAtLoad</key>
		<true/>

		<key>KeepAlive</key>
		<true/>
	</dict>
</plist>
`, macPlistName, nixAgentBin, nixAgentDir)

		os.WriteFile(macPlistPath, []byte(svc), 0644)
		opts := a.NewCMDOpts()
		opts.Command = fmt.Sprintf("launchctl bootstrap system %s", macPlistPath)
		a.CmdV2(opts)
	}

	if runtime.GOOS == "windows" {
		os.MkdirAll(filepath.Join(a.ProgramDir, "bin"), 0755)

		// send software api
		a.SendSoftware()

		a.Logger.Debugln("Creating temp dir")
		err := createWinTempDir()
		if err != nil {
			a.Logger.Errorln("Install() createWinTempDir():", err)
		}

		a.Logger.Debugln("Disabling automatic windows updates")
		a.PatchMgmnt(true)

		a.Logger.Infoln("Installing service...")
		err = a.InstallService()
		if err != nil {
			a.installerMsg(err.Error(), "error", i.Silent)
		}

		time.Sleep(1 * time.Second)
		a.Logger.Infoln("Starting service...")
		out := a.ControlService(winSvcName, "start")
		if !out.Success {
			a.installerMsg(out.ErrorMsg, "error", i.Silent)
		}

		if i.Power {
			a.Logger.Infoln("Disabling sleep/hibernate...")
			DisableSleepHibernate()
		}

		if i.Ping {
			a.Logger.Infoln("Enabling ping...")
			EnablePing()
		}

		if i.RDP {
			a.Logger.Infoln("Enabling RDP...")
			EnableRDP()
		}
	}

	a.installerMsg("Installation was successful!\nAllow a few minutes for the agent to properly display in the RMM", "info", i.Silent)
}

// generateRandomString creates a random alphanumeric string of given length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}

// verifyMeshFiles reads SHA256SUMS and verifies the integrity of mesh files
// Returns the base filename (e.g., "meshagent93c61773") or error
func verifyMeshFiles(extractDir string) (string, error) {
	sha256sumsPath := filepath.Join(extractDir, "SHA256SUMS")

	// Read SHA256SUMS file
	file, err := os.Open(sha256sumsPath)
	if err != nil {
		return "", fmt.Errorf("failed to open SHA256SUMS: %w", err)
	}
	defer file.Close()

	// Parse SHA256SUMS file (format: "<hash>  <filename>")
	checksums := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		checksums[parts[1]] = parts[0]
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read SHA256SUMS: %w", err)
	}

	// Find meshagent binary filename
	var meshBinary, meshMSH string
	for filename := range checksums {
		if strings.HasPrefix(filename, "meshagent") && !strings.HasSuffix(filename, ".msh") {
			meshBinary = filename
		} else if strings.HasPrefix(filename, "meshagent") && strings.HasSuffix(filename, ".msh") {
			meshMSH = filename
		}
	}

	if meshBinary == "" || meshMSH == "" {
		return "", fmt.Errorf("could not find meshagent files in SHA256SUMS")
	}

	// Verify checksums
	for _, filename := range []string{meshBinary, meshMSH} {
		filePath := filepath.Join(extractDir, filename)
		expectedHash := checksums[filename]

		// Calculate actual hash
		f, err := os.Open(filePath)
		if err != nil {
			return "", fmt.Errorf("failed to open %s: %w", filename, err)
		}
		defer f.Close()

		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return "", fmt.Errorf("failed to hash %s: %w", filename, err)
		}
		actualHash := hex.EncodeToString(h.Sum(nil))

		if actualHash != expectedHash {
			return "", fmt.Errorf("checksum mismatch for %s: expected %s, got %s", filename, expectedHash, actualHash)
		}
	}

	// Return base filename (without .msh extension)
	return meshBinary, nil
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
