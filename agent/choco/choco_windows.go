package choco

import (
	"fmt"
	"time"

	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/tactical/api"
	"github.com/amidaware/rmmagent/agent/tactical/config"
	"github.com/go-resty/resty/v2"
)

func InstallChoco() error {
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
		return err
	}

	if r.IsError() {
		api.PostPayload(result, url)
		return fmt.Errorf("response code: %d", r.StatusCode())
	}

	installScript := string(r.Body())
	_, _, exitcode, err := system.RunScript(installScript, "powershell", []string{}, 900)
	if err != nil {
		api.PostPayload(result, url)
		return err
	}

	if exitcode != 0 {
		api.PostPayload(result, url)
		return fmt.Errorf("exit code: %d", exitcode)
	}

	result.Installed = true
	err = api.PostPayload(result, url)

	return err
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
