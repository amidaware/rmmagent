package agent

import (
	"fmt"
	"log"
	"io/ioutil"
	"net/http"
	"encoding/json"
	"os/exec"
	"runtime"
	"syscall"
	"strings"
	"time"
	"os"
	"io"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"github.com/fourcorelabs/wintoken"
)

type SupportConfigItem struct {
	Type       string `json:"type"`
	Title      string `json:"title,omitempty"`
	Action     string `json:"action,omitempty"`
	Tooltip    string `json:"tooltip,omitempty"`
	IsChecked  bool   `json:"isChecked,omitempty"`
	IsDisabled bool   `json:"isDisabled,omitempty"`
}

type SupportConfig struct {
	Type   string             `json:"type"`
	Items  []SupportConfigItem `json:"items"`
	Title  string             `json:"title,omitempty"`
	Tooltip string            `json:"tooltip,omitempty"`
}

type SupportResponse struct {
	SupportConfig []SupportConfig `json:"support_config"`
	SystrayEnabled bool           `json:"systray_enabled"`
	SupportIcon    string         `json:"support_icon"`
	SupportName    string         `json:"support_name"`
}

func (a *Agent) GetSystrayConfig() {
	if runtime.GOOS != "windows" {
		log.Println("System trays are only supported on windows at this time.")
		return
	}

	reg, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\TacticalRMM`, registry.QUERY_VALUE)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer reg.Close()

	agentID, _, err := reg.GetStringValue("agentID")
	if err != nil {
		fmt.Println(err)
		return
	}

	baseURL, _, err := reg.GetStringValue("BaseURL")
	if err != nil {
		fmt.Println(err)
		return
	}

	token, _, err := reg.GetStringValue("Token")
	if err != nil {
		fmt.Println(err)
		return
	}

	// Construct request URL and create the request
	requestURL := fmt.Sprintf("%s/api/v3/%s/support/", baseURL, agentID)
	req, err := http.NewRequest("GET", requestURL, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}
	
	// Add the Authorization header
	req.Header.Add("Authorization", "Token "+token)
	
	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()
	
	// Read the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return
	}
	
	var result SupportResponse
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println("Error unmarshaling response:", err)
		return
	}

	fmt.Printf("Support Information: %+v\n", result)

	// Close existing systray if it exists
	closeExistingSystray()

	// Check if support is enabled
	if result.SystrayEnabled {
		// Task 1: Download the icon
		iconErr := downloadIcon(result.SupportIcon, agentID, token)
		if iconErr != nil {
			fmt.Println("Error downloading icon:", iconErr)
			return
		}
	
		// Task 2: Start the systray application
		startSystray()
		time.Sleep(1 * time.Second)

		// Task 3: Send config over named pipe
		sendErr := SendConfigToNamedPipe(result.SupportName, result.SupportConfig)
		if sendErr != nil {
			fmt.Println("Error sending config to named pipe:", sendErr)
			return
		}
	}
}

func downloadIcon(url, agentId, token string) error {
	// Correct the URL by inserting the agent ID
	correctedURL := strings.Replace(url, "/support/", fmt.Sprintf("/%s/%s/%s/support/", "api", "v3", agentId), 1)

	// Create the HTTP request
	req, err := http.NewRequest("GET", correctedURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %v", err)
	}

	// Add the Authorization header
	req.Header.Add("Authorization", "Token "+token)

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %v", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("non-OK HTTP status: %s", resp.Status)
	}

	// Ensure the target directory exists
	iconDir := `C:\ProgramData\TacticalRMM`
	if err := os.MkdirAll(iconDir, 0755); err != nil {
		return fmt.Errorf("creating directory: %v", err)
	}

	// Create the file
	filePath := iconDir + `\icon.ico`
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("creating file: %v", err)
	}
	defer file.Close()

	// Write the body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("writing to file: %v", err)
	}

	fmt.Printf("Icon successfully downloaded to %s\n", filePath)
	return nil
}

func startSystray() {
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
	cmd := exec.Command("C:\\Program Files\\TacticalAgent\\tacticalrmm.exe", "-m", "tray")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token: syscall.Token(token.Token()),
	}
	if err := cmd.Start(); err != nil {
		log.Println("Failed to start tray application: %v", err)
	} else {
		log.Println("Tray application launched.")
	}
}

func SendConfigToNamedPipe(supportName string, config []SupportConfig) error {

	pipeName := `\\.\pipe\TRMM`

	// Open the named pipe
	hPipe, err := windows.CreateFile(
		syscall.StringToUTF16Ptr(pipeName),
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil,
		windows.OPEN_EXISTING,
		0, 0)
	if err != nil {
		return fmt.Errorf("failed to open named pipe: %v", err)
	}
	defer windows.CloseHandle(hPipe)

	// Prepare the data to send
	data := struct {
		Name   string          `json:"name"`
		Config []SupportConfig `json:"config"`
	}{
		Name:   supportName,
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
		return fmt.Println("failed to write to named pipe: %v", err)
	}

	fmt.Println("Data sent to named pipe successfully.")
	return nil
}

func closeExistingSystray() {
	const pipeName = `\\.\pipe\TRMM`
	var securityAttributes windows.SecurityAttributes

	// Open the named pipe
	hPipe, err := windows.CreateFile(
		syscall.StringToUTF16Ptr(pipeName),
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0,
		&securityAttributes,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		fmt.Printf("Failed to open named pipe: %v\n", err)
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
		fmt.Printf("Failed to marshal message: %v\n", err)
		return
	}

	// Write the message to the pipe
	var written uint32
	err = windows.WriteFile(hPipe, msgBytes, &written, nil)
	if err != nil {
		fmt.Printf("Failed to write to named pipe: %v\n", err)
		return
	}

	fmt.Println("Message sent successfully.")
}