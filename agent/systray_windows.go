package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/fourcorelabs/wintoken"
	"golang.org/x/sys/windows"
)

type SystrayConfigItem struct {
	Type       string `json:"type"`
	Title      string `json:"title,omitempty"`
	Action     string `json:"action,omitempty"`
	Tooltip    string `json:"tooltip,omitempty"`
	IsChecked  bool   `json:"isChecked,omitempty"`
	IsDisabled bool   `json:"isDisabled,omitempty"`
}

type SystrayConfig struct {
	Type    string              `json:"type"`
	Items   []SystrayConfigItem `json:"items"`
	Title   string              `json:"title,omitempty"`
	Action  string              `json:"action"`
	Tooltip string              `json:"tooltip,omitempty"`
}

type SystrayResponse struct {
	SystrayConfig  []SystrayConfig `json:"systray_config"`
	SystrayEnabled bool            `json:"systray_enabled"`
	SystrayIcon    string          `json:"systray_icon"`
	SystrayName    string          `json:"systray_name"`
}

func (a *Agent) GetSystrayConfig() {
	if runtime.GOOS != "windows" {
		a.Logger.Debugln("System trays are only supported on windows at this time.")
		return
	}

	// Send the request
	url := fmt.Sprintf("/api/v3/%s/systray/", a.AgentID)
	resp, err := a.rClient.R().Get(url)
	if err != nil {
		a.Logger.Debugln("Error sending request:", err)
		return
	}

	// Read the response
	body := resp.Body()
	if body == nil {
		a.Logger.Debugln("Error: response body is nil")
		return
	}

	var result SystrayResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		a.Logger.Debugln("Invalid or Missing Config:", err)
		a.closeExistingSystray()
		return
	}

	a.Logger.Debugln("Systray Information:", result)

	// Close existing systray if it exists
	a.closeExistingSystray()

	// Check if systray is enabled
	if result.SystrayEnabled {
		// Task 1: Download the icon
		iconErr := a.downloadTrayIcon(result.SystrayIcon, a.AgentID)
		if iconErr != nil {
			a.Logger.Debugln("Error downloading icon:", iconErr)
			return
		}

		// Task 2: Start the systray application
		startSystray(a.EXE)
		time.Sleep(1 * time.Second)

		// Task 3: Send config over named pipe
		sendErr := SendConfigToNamedPipe(result.SystrayName, result.SystrayConfig)
		if sendErr != nil {
			a.Logger.Debugln("Error sending config to named pipe:", sendErr)
			return
		}
	}
}

func (a *Agent) downloadTrayIcon(url, agentId string) error {
	// Correct the URL by inserting the agent ID
	correctedURL := strings.Replace(url, "/core/systray/", fmt.Sprintf("/%s/%s/%s/systray/", "api", "v3", agentId), 1)

	// Execute the request
	resp, err := a.rClient.SetBaseURL("").R().Get(correctedURL)
	if err != nil {
		a.Logger.Debugln("executing request:", err)
		return err
	}

	a.rClient.SetBaseURL(a.BaseURL)

	// Ensure the target directory exists
	iconDir := filepath.Join(os.Getenv("ProgramData"), "TacticalRMM")
	if err := os.MkdirAll(iconDir, 0755); err != nil {
		a.Logger.Debugln("creating directory:", err)
		return err
	}

	// Create the file
	filePath := iconDir + `\icon.ico`
	file, err := os.Create(filePath)
	if err != nil {
		a.Logger.Debugln("creating file:", err)
		return err
	}
	defer file.Close()

	// Read the response
	body := resp.Body()
	if body == nil {
		a.Logger.Debugln("Error: response body is nil")
		return nil
	}

	// Convert the body to an io.Reader
	reader := bytes.NewReader(body)

	// Write the body to file
	_, err = io.Copy(file, reader)
	if err != nil {
		a.Logger.Debugln("writing to file:", err)
		return err
	}

	a.Logger.Debugln("Icon successfully downloaded to", filePath)
	return nil
}

func startSystray(exe string) {
	if runtime.GOOS != "windows" {
		log.Println("This function is designed to run on Windows.")
		return
	}

	// Fetch an interactive token.
	token, err := wintoken.GetInteractiveToken(wintoken.TokenImpersonation)
	if err != nil {
		log.Printf("Failed to get interactive token: %v", err)
	}
	defer token.Close()

	// Launch the tray application using the fetched token.
	cmd := exec.Command(exe, "-m", "tray")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token: syscall.Token(token.Token()),
	}
	if err := cmd.Start(); err != nil {
		log.Println("Failed to start tray application: %v", err)
	} else {
		log.Println("Tray application launched.")
	}
}

func SendConfigToNamedPipe(systrayName string, config []SystrayConfig) error {
	pipeName := `\\.\pipe\TRMM`

	// Retry mechanism for opening the named pipe
	var hPipe windows.Handle
	var err error
	for retries := 0; retries < 5; retries++ {
		hPipe, err = windows.CreateFile(
			syscall.StringToUTF16Ptr(pipeName),
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0, nil,
			windows.OPEN_EXISTING,
			0, 0)
		if err == nil {
			break
		}
		log.Printf("Failed to open named pipe on attempt", retries+1, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return fmt.Errorf("failed to open named pipe:", err)
	}
	defer windows.CloseHandle(hPipe)

	// Prepare the data to send
	data := struct {
		Name   string          `json:"name"`
		Config []SystrayConfig `json:"config"`
	}{
		Name:   systrayName,
		Config: config,
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to serialize data: %v", err)
	}

	// Write data to the named pipe
	var written uint32
	err = windows.WriteFile(hPipe, dataBytes, &written, nil)
	if err != nil {
		return fmt.Errorf("failed to write to named pipe: %v", err)
	}

	log.Println("Data sent to named pipe successfully.")
	return nil
}

func (a *Agent) closeExistingSystray() {
	const pipeName = `\\.\pipe\TRMM`

	var hPipe windows.Handle
	var err error
	for retries := 0; retries < 5; retries++ {
		hPipe, err = windows.CreateFile(
			syscall.StringToUTF16Ptr(pipeName),
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0, nil,
			windows.OPEN_EXISTING,
			0, 0)
		if err == nil {
			break
		}
		a.Logger.Debugln("Failed to open named pipe on attempt", retries+1, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		a.Logger.Debugln("Failed to open named pipe:", err)
		return
	}
	defer windows.CloseHandle(hPipe)

	// Construct the message to send
	msg := struct {
		Name string `json:"name"`
	}{
		Name: "systray-shutdown",
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		a.Logger.Debugln("Failed to marshal message:", err)
		return
	}

	// Write the message to the pipe
	var written uint32
	err = windows.WriteFile(hPipe, msgBytes, &written, nil)
	if err != nil {
		a.Logger.Debugln("Failed to write to named pipe:", err)
		return
	}

	a.Logger.Debugln("Message sent successfully.")
}
