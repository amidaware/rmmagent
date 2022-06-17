package services

import (
	trmm "github.com/wh1te909/trmm-shared"
	rmm "github.com/amidaware/rmmagent/shared"
	"github.com/kardianos/service"
)

func GetServiceDetail(name string) trmm.WindowsService { return trmm.WindowsService{} }

func ControlService(name, action string) rmm.WinSvcResp {
	return rmm.WinSvcResp{Success: false, ErrorMsg: "/na"}
}

func EditService(name, startupType string) rmm.WinSvcResp {
	return rmm.WinSvcResp{Success: false, ErrorMsg: "/na"}
}

func GetServices() []trmm.WindowsService { return []trmm.WindowsService{} }

func Start(_ service.Service) error { return nil }

func Stop(_ service.Service) error { return nil }

func InstallService() error { return nil }