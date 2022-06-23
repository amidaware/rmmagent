package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/amidaware/rmmagent/agent/tactical/config"
	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/go-resty/resty/v2"
)

const (
	ProgFilesName = "TacticalAgent"
	winExeName    = "tacticalrmm.exe"
)

func GetPython(force bool) {
	if utils.FileExists(GetPythonBin()) && !force {
		return
	}

	var archZip string
	var folder string
	switch runtime.GOARCH {
	case "amd64":
		archZip = "py38-x64.zip"
		folder = "py38-x64"
	case "386":
		archZip = "py38-x32.zip"
		folder = "py38-x32"
	}
	pyFolder := filepath.Join(GetProgramDirectory(), folder)
	pyZip := filepath.Join(GetProgramDirectory(), archZip)
	defer os.Remove(pyZip)

	if force {
		os.RemoveAll(pyFolder)
	}

	config := config.NewAgentConfig()
	rClient := resty.New()
	rClient.SetTimeout(20 * time.Minute)
	rClient.SetRetryCount(10)
	rClient.SetRetryWaitTime(1 * time.Minute)
	rClient.SetRetryMaxWaitTime(15 * time.Minute)
	if len(config.Proxy) > 0 {
		rClient.SetProxy(config.Proxy)
	}

	url := fmt.Sprintf("https://github.com/amidaware/rmmagent/releases/download/v2.0.0/%s", archZip)
	r, err := rClient.R().SetOutput(pyZip).Get(url)
	if err != nil {
		return
	}
	if r.IsError() {
		return
	}

	err = utils.Unzip(pyZip, GetProgramDirectory())
	if err != nil {
	}
}

func RunMigrations() {
	for _, i := range []string{"nssm.exe", "nssm-x86.exe"} {
		nssm := filepath.Join(GetProgramDirectory(), i)
		if utils.FileExists(nssm) {
			os.Remove(nssm)
		}
	}
}

func GetPythonBin() string {
	var pybin string
	switch runtime.GOARCH {
	case "amd64":
		pybin = filepath.Join(GetProgramDirectory(), "py38-x64", "python.exe")
	case "386":
		pybin = filepath.Join(GetProgramDirectory(), "py38-x32", "python.exe")
	}

	return pybin
}

func GetProgramDirectory() string {
	pd := filepath.Join(os.Getenv("ProgramFiles"), ProgFilesName)
	return pd
}

func GetProgramBin() string {
	exe := filepath.Join(GetProgramDirectory(), winExeName)
	return exe
}
