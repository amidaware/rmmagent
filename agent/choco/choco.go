package choco

import (
	"time"

	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/tactical/api"
	"github.com/amidaware/rmmagent/agent/tactical/config"
	"github.com/go-resty/resty/v2"
)

func InstallChoco() {
	config := config.NewAgentConfig()
	var result ChocoInstalled
	result.AgentID = config.AgentID
	result.Installed = false

	rClient := resty.New()
	rClient.SetTimeout(30 * time.Second)
	if len(config.Proxy) > 0 {
		rClient.SetProxy(config.Proxy)
	}

	url := "/api/v3/choco/"
	r, err := rClient.R().Get("https://chocolatey.org/install.ps1")
	if err != nil {
		api.PostPayload(result, url)
		return
	}

	if r.IsError() {
		api.PostPayload(result, url)
		return
	}

	_, _, exitcode, err := system.RunScript(string(r.Body()), "powershell", []string{}, 900)
	if err != nil {
		api.PostPayload(result, url)
		return
	}

	if exitcode != 0 {
		api.PostPayload(result, url)
		return
	}

	result.Installed = true
	api.PostPayload(result, url)
}

func InstallWithChoco(name string) (string, error) {
	out, err := system.CMD("choco.exe", []string{"install", name, "--yes", "--force", "--force-dependencies", "--no-progress"}, 1200, false)
	if err != nil {
		return err.Error(), err
	}

	if out[1] != "" {
		return out[1], nil
	}

	return out[0], nil
}
