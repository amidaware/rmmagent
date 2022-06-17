package services

import (
	"fmt"

	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/gonutz/w32/v2"
	trmm "github.com/wh1te909/trmm-shared"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	WinSvcName  = "tacticalrmm"
	meshSvcName = "mesh agent"
)

// ShowStatus prints windows service status
// If called from an interactive desktop, pops up a message box
// Otherwise prints to the console
func ShowStatus(version string) {
	statusMap := make(map[string]string)
	svcs := []string{WinSvcName, meshSvcName}

	for _, service := range svcs {
		status, err := GetServiceStatus(service)
		if err != nil {
			statusMap[service] = "Not Installed"
			continue
		}
		statusMap[service] = status
	}

	window := w32.GetForegroundWindow()
	if window != 0 {
		_, consoleProcID := w32.GetWindowThreadProcessId(window)
		if w32.GetCurrentProcessId() == consoleProcID {
			w32.ShowWindow(window, w32.SW_HIDE)
		}
		var handle w32.HWND
		msg := fmt.Sprintf("Agent: %s\n\nMesh Agent: %s", statusMap[WinSvcName], statusMap[meshSvcName])
		w32.MessageBox(handle, msg, fmt.Sprintf("Tactical RMM v%s", version), w32.MB_OK|w32.MB_ICONINFORMATION)
	} else {
		fmt.Println("Tactical RMM Version", version)
		fmt.Println("Tactical Agent:", statusMap[WinSvcName])
		fmt.Println("Mesh Agent:", statusMap[meshSvcName])
	}
}

func GetServiceStatus(name string) (string, error) {
	conn, err := mgr.Connect()
	if err != nil {
		return "n/a", err
	}
	defer conn.Disconnect()

	srv, err := conn.OpenService(name)
	if err != nil {
		return "n/a", err
	}
	defer srv.Close()

	q, err := srv.Query()
	if err != nil {
		return "n/a", err
	}

	return serviceStatusText(uint32(q.State)), nil
}

// https://docs.microsoft.com/en-us/dotnet/api/system.serviceprocess.servicecontrollerstatus?view=dotnet-plat-ext-3.1
func serviceStatusText(num uint32) string {
	switch num {
	case 1:
		return "stopped"
	case 2:
		return "start_pending"
	case 3:
		return "stop_pending"
	case 4:
		return "running"
	case 5:
		return "continue_pending"
	case 6:
		return "pause_pending"
	case 7:
		return "paused"
	default:
		return "unknown"
	}
}

// GetServices returns a list of windows services
func GetServices() []trmm.WindowsService {
	ret := make([]trmm.WindowsService, 0)

	conn, err := mgr.Connect()
	if err != nil {
		//a.Logger.Debugln(err)
		return ret
	}
	defer conn.Disconnect()

	svcs, err := conn.ListServices()

	if err != nil {
		//a.Logger.Debugln(err)
		return ret
	}

	for _, s := range svcs {
		srv, err := conn.OpenService(s)
		if err != nil {
			if err.Error() != "Access is denied." {
				//a.Logger.Debugln("Open Service", s, err)
			}

			continue
		}

		defer srv.Close()
		q, err := srv.Query()
		if err != nil {
			//a.Logger.Debugln(err)
			continue
		}

		conf, err := srv.Config()
		if err != nil {
			//a.Logger.Debugln(err)
			continue
		}

		ret = append(ret, trmm.WindowsService{
			Name:             s,
			Status:           serviceStatusText(uint32(q.State)),
			DisplayName:      utils.CleanString(conf.DisplayName),
			BinPath:          utils.CleanString(conf.BinaryPathName),
			Description:      utils.CleanString(conf.Description),
			Username:         utils.CleanString(conf.ServiceStartName),
			PID:              q.ProcessId,
			StartType:        serviceStartType(uint32(conf.StartType)),
			DelayedAutoStart: conf.DelayedAutoStart,
		})
	}
	return ret
}

// https://docs.microsoft.com/en-us/dotnet/api/system.serviceprocess.servicestartmode?view=dotnet-plat-ext-3.1
func serviceStartType(num uint32) string {
	switch num {
	case 0:
		return "Boot"
	case 1:
		return "System"
	case 2:
		return "Automatic"
	case 3:
		return "Manual"
	case 4:
		return "Disabled"
	default:
		return "Unknown"
	}
}
