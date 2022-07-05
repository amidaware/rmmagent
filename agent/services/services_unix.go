//go:build !windows
// +build !windows

package services

import (
	"github.com/kardianos/service"
)

func GetServiceDetail(name string) Service { return Service{} }

func ControlService(name, action string) WinSvcResp {
	return WinSvcResp{Success: false, ErrorMsg: "/na"}
}

func EditService(name, startupType string) WinSvcResp {
	return WinSvcResp{Success: false, ErrorMsg: "/na"}
}

func GetServices() ([]Service, []error, error) {
	return []Service{}, []error{}, nil
}

func Start(_ service.Service) error { return nil }

func Stop(_ service.Service) error { return nil }

func InstallService() error { return nil }

func GetServiceStatus(name string) (string, error) {
	return "", nil
}