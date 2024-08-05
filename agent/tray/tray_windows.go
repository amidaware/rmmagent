package tray

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/getlantern/systray"
)

var shutdownChan = make(chan struct{})
var configItems []MenuItemConfig

type MenuItemConfig struct {
	Type       string           `json:"type"`
	Title      string           `json:"title"`
	Tooltip    string           `json:"tooltip"`
	IsDisabled *bool            `json:"isDisabled,omitempty"` // Optional
	IsHidden   *bool            `json:"isHidden,omitempty"`   // Optional
	IsChecked  *bool            `json:"isChecked,omitempty"`  // Optional
	Action     string           `json:"action"`
	Items      []MenuItemConfig `json:"items,omitempty"`
}

var kernel32 = syscall.NewLazyDLL("kernel32.dll")

func setConsoleTitle(title string) {
	proc := kernel32.NewProc("SetConsoleTitleW")
	utf16Title := syscall.StringToUTF16(title)
	proc.Call(uintptr(unsafe.Pointer(&utf16Title[0])))
}

func InitTrayIcon() {
	go func() {
		<-shutdownChan
		performCleanup()
		systray.Quit()
	}()

	go startPipe()
	setConsoleTitle("Tray")
	systray.Run(onReady, onExit)
}

func onReady() {
	log.Println("onReady executed")

	iconPath := filepath.Join(os.Getenv("ProgramData"), "TacticalRMM", "icon.ico")
	iconData, err := os.ReadFile(iconPath)
	if err != nil {
		log.Printf("Failed to read icon file: %v", err)
		iconData = iconTRMM
	}

	var currentTitle string
	currentTitle = "Tactical RMM"
	systray.SetIcon(iconData)
	systray.SetTitle("Systray")
	systray.SetTooltip(currentTitle)

	// Read and parse JSON config file
	configPath := filepath.Join(os.Getenv("ProgramFiles"), "TacticalAgent", "config.json")
	configFile, err := os.ReadFile(configPath)
	if err != nil {
		log.Printf("Error reading config file: %v", err)
		return
	}

	err = json.Unmarshal(configFile, &configItems)
	if err != nil {
		log.Printf("Error parsing config file: %v", err)
		return
	}

	// Add items to tray based on config
	for _, item := range configItems {
		addItemToTray(item)
	}
}

func addItemToTray(item MenuItemConfig) {
	switch item.Type {
	case "link":
		menuItem := systray.AddMenuItem(item.Title, item.Tooltip)
		menuItem.SetTitle(item.Title)
		menuItem.SetTooltip(item.Tooltip)
		if item.IsDisabled != nil && *item.IsDisabled {
			menuItem.Disable()
		}
		if item.IsChecked != nil {
			if *item.IsChecked {
				menuItem.Check()
			} else {
				menuItem.Uncheck()
			}
		}
		go func() {
			for {
				select {
				case <-menuItem.ClickedCh:
					handleLinkAction(item.Action)
				}
			}
		}()
	case "submenu":
		parentItem := systray.AddMenuItem(item.Title, item.Tooltip)
		parentItem.SetTitle(item.Title)
		parentItem.SetTooltip(item.Tooltip)
		if item.IsDisabled != nil && *item.IsDisabled {
			parentItem.Disable()
		}
		for _, subItem := range item.Items {
			addItemToTrayHelper(parentItem, subItem)
		}
	case "divider":
		systray.AddSeparator()
	}
}

func addItemToTrayHelper(parent *systray.MenuItem, item MenuItemConfig) {
	switch item.Type {
	case "link":
		menuItem := parent.AddSubMenuItem(item.Title, item.Tooltip)
		menuItem.SetTitle(item.Title)
		menuItem.SetTooltip(item.Tooltip)
		if item.IsDisabled != nil && *item.IsDisabled {
			menuItem.Disable()
		}
		if item.IsChecked != nil {
			if *item.IsChecked {
				menuItem.Check()
			} else {
				menuItem.Uncheck()
			}
		}
		go func() {
			for {
				select {
				case <-menuItem.ClickedCh:
					handleLinkAction(item.Action)
				}
			}
		}()
	case "submenu":
		subParentItem := parent.AddSubMenuItem(item.Title, item.Tooltip)
		subParentItem.SetTitle(item.Title)
		subParentItem.SetTooltip(item.Tooltip)
		if item.IsDisabled != nil && *item.IsDisabled {
			subParentItem.Disable()
		}
		for _, subItem := range item.Items {
			addItemToTrayHelper(subParentItem, subItem)
		}
	}
}

func handleLinkAction(action string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", action)
	case "windows":
		log.Println(action)
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", action)
	case "darwin":
		cmd = exec.Command("open", action)
	default:
		log.Printf("Unsupported platform")
		return
	}

	err := cmd.Start()
	if err != nil {
		log.Printf("Failed to start URL open command: %s", err)
		return
	}

	// Wait for the command to complete
	err = cmd.Wait()
	if err != nil {
		log.Printf("Command finished with error: %s", err)
	}
}

func onExit() {

}

func performCleanup() {
	closePipe()
}

func updateSystray() {
	// Safely update the systray title
	systray.SetTooltip(currentTitle)

	if !isEffectivelyEmpty(configItems) {
		for _, item := range currentConfig {
			addItemToTray(item)
		}
	}
}

func isEffectivelyEmpty(items []MenuItemConfig) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if item.Type != "" || len(item.Items) > 0 {
			return true // Found a non-empty item
		}
	}
	return false
}
