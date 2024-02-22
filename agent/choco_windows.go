/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	rmm "github.com/amidaware/rmmagent/shared"
	"github.com/go-resty/resty/v2"
)

func (a *Agent) InstallChoco() {

	var result rmm.ChocoInstalled
	result.AgentID = a.AgentID
	result.Installed = false

	rClient := resty.New()
	rClient.SetTimeout(30 * time.Second)
	if len(a.Proxy) > 0 {
		rClient.SetProxy(a.Proxy)
	}

	url := "/api/v3/choco/"
	r, err := rClient.R().Get("https://chocolatey.org/install.ps1")
	if err != nil {
		a.Logger.Debugln(err)
		a.rClient.R().SetBody(result).Post(url)
		return
	}
	if r.IsError() {
		a.rClient.R().SetBody(result).Post(url)
		return
	}

	_, _, exitcode, err := a.RunScript(string(r.Body()), "powershell", []string{}, 900, false, []string{}, false, "")
	if err != nil {
		a.Logger.Debugln(err)
		a.rClient.R().SetBody(result).Post(url)
		return
	}

	if exitcode != 0 {
		a.rClient.R().SetBody(result).Post(url)
		return
	}

	result.Installed = true
	a.rClient.R().SetBody(result).Post(url)
}

func (a *Agent) InstallWithChoco(name string) (string, error) {
	var exe string
	choco, err := exec.LookPath("choco.exe")
	if err != nil || choco == "" {
		exe = filepath.Join(os.Getenv("PROGRAMDATA"), `chocolatey\bin\choco.exe`)
	} else {
		exe = choco
	}
	out, err := CMD(exe, []string{"install", name, "--yes", "--force", "--force-dependencies", "--no-progress"}, 1200, false)
	if err != nil {
		a.Logger.Errorln(err)
		return err.Error(), err
	}
	if out[1] != "" {
		return out[1], nil
	}
	return out[0], nil
}
