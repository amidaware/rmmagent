package services

import (
	"fmt"
	"time"

	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/gonutz/w32/v2"
	"golang.org/x/sys/windows/svc"
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
func GetServices() ([]Service, []error, error) {
	ret := make([]Service, 0)

	conn, err := mgr.Connect()
	if err != nil {
		return ret, nil, err
	}

	defer conn.Disconnect()
	svcs, err := conn.ListServices()
	if err != nil {
		return ret, nil, err
	}

	errors := []error{}

	for _, s := range svcs {
		srv, err := conn.OpenService(s)
		if err != nil {
			if err.Error() != "Access is denied." {
				//a.Logger.Debugln("Open Service", s, err)
				errors = append(errors, err)
			}

			continue
		}

		defer srv.Close()
		q, err := srv.Query()
		if err != nil {
			errors = append(errors, err)
			continue
		}

		conf, err := srv.Config()
		if err != nil {
			errors = append(errors, err)
			continue
		}

		ret = append(ret, Service{
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

	return ret, errors, nil
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

func ControlService(name, action string) WinSvcResp {
	conn, err := mgr.Connect()
	if err != nil {
		return WinSvcResp{Success: false, ErrorMsg: err.Error()}
	}

	defer conn.Disconnect()
	srv, err := conn.OpenService(name)
	if err != nil {
		return WinSvcResp{Success: false, ErrorMsg: err.Error()}
	}

	defer srv.Close()
	var status svc.Status
	switch action {

	case "stop":
		status, err = srv.Control(svc.Stop)
		if err != nil {
			return WinSvcResp{Success: false, ErrorMsg: err.Error()}
		}
		timeout := time.Now().Add(30 * time.Second)
		for status.State != svc.Stopped {
			if timeout.Before(time.Now()) {
				return WinSvcResp{Success: false, ErrorMsg: "Timed out waiting for service to stop"}
			}

			time.Sleep(500 * time.Millisecond)
			status, err = srv.Query()
			if err != nil {
				return WinSvcResp{Success: false, ErrorMsg: err.Error()}
			}
		}

		return WinSvcResp{Success: true, ErrorMsg: ""}

	case "start":
		err := srv.Start()
		if err != nil {
			return WinSvcResp{Success: false, ErrorMsg: err.Error()}
		}

		return WinSvcResp{Success: true, ErrorMsg: ""}
	}

	return WinSvcResp{Success: false, ErrorMsg: "Something went wrong"}
}

func GetServiceDetail(name string) Service {
	ret := Service{}

	conn, err := mgr.Connect()
	if err != nil {
		return ret
	}

	defer conn.Disconnect()
	srv, err := conn.OpenService(name)
	if err != nil {
		return ret
	}

	defer srv.Close()
	q, err := srv.Query()
	if err != nil {
		return ret
	}

	conf, err := srv.Config()
	if err != nil {
		return ret
	}

	ret.BinPath = utils.CleanString(conf.BinaryPathName)
	ret.Description = utils.CleanString(conf.Description)
	ret.DisplayName = utils.CleanString(conf.DisplayName)
	ret.Name = name
	ret.PID = q.ProcessId
	ret.StartType = serviceStartType(uint32(conf.StartType))
	ret.Status = serviceStatusText(uint32(q.State))
	ret.Username = utils.CleanString(conf.ServiceStartName)
	ret.DelayedAutoStart = conf.DelayedAutoStart
	return ret
}

func EditService(name, startupType string) WinSvcResp {
	conn, err := mgr.Connect()
	if err != nil {
		return WinSvcResp{Success: false, ErrorMsg: err.Error()}
	}
	defer conn.Disconnect()

	srv, err := conn.OpenService(name)
	if err != nil {
		return WinSvcResp{Success: false, ErrorMsg: err.Error()}
	}
	defer srv.Close()

	conf, err := srv.Config()
	if err != nil {
		return WinSvcResp{Success: false, ErrorMsg: err.Error()}
	}

	var startType uint32
	switch startupType {
	case "auto":
		startType = 2
	case "autodelay":
		startType = 2
	case "manual":
		startType = 3
	case "disabled":
		startType = 4
	default:
		return WinSvcResp{Success: false, ErrorMsg: "Unknown startup type provided"}
	}

	conf.StartType = startType
	switch startupType {
	case "autodelay":
		conf.DelayedAutoStart = true
	case "auto":
		conf.DelayedAutoStart = false
	}

	err = srv.UpdateConfig(conf)
	if err != nil {
		return WinSvcResp{Success: false, ErrorMsg: err.Error()}
	}

	return WinSvcResp{Success: true, ErrorMsg: ""}
}

func ServiceExists(name string) (bool, error) {
	conn, err := mgr.Connect()
	if err != nil {
		return false, err
	}

	defer conn.Disconnect()
	srv, err := conn.OpenService(name)
	if err != nil {
		return false, err
	}

	defer srv.Close()
	return true, nil
}
