/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gonutz/w32/v2"
	"golang.org/x/sys/windows/registry"
)

func createAgentConfig(baseurl, agentid, apiurl, token, agentpk, cert, proxy, meshdir, natsport string, insecure bool) {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\TacticalRMM`, registry.ALL_ACCESS)
	if err != nil {
		log.Fatalln("Error creating registry key:", err)
	}
	defer k.Close()

	err = k.SetStringValue("BaseURL", baseurl)
	if err != nil {
		log.Fatalln("Error creating BaseURL registry key:", err)
	}

	err = k.SetStringValue("AgentID", agentid)
	if err != nil {
		log.Fatalln("Error creating AgentID registry key:", err)
	}

	err = k.SetStringValue("ApiURL", apiurl)
	if err != nil {
		log.Fatalln("Error creating ApiURL registry key:", err)
	}

	err = k.SetStringValue("Token", token)
	if err != nil {
		log.Fatalln("Error creating Token registry key:", err)
	}

	err = k.SetStringValue("AgentPK", agentpk)
	if err != nil {
		log.Fatalln("Error creating AgentPK registry key:", err)
	}

	if len(cert) > 0 {
		err = k.SetStringValue("Cert", cert)
		if err != nil {
			log.Fatalln("Error creating Cert registry key:", err)
		}
	}

	if len(proxy) > 0 {
		err = k.SetStringValue("Proxy", proxy)
		if err != nil {
			log.Fatalln("Error creating Proxy registry key:", err)
		}
	}

	if len(meshdir) > 0 {
		err = k.SetStringValue("MeshDir", meshdir)
		if err != nil {
			log.Fatalln("Error creating MeshDir registry key:", err)
		}
	}

	if len(natsport) > 0 {
		err = k.SetStringValue("NatsStandardPort", natsport)
		if err != nil {
			log.Fatalln("Error creating NatsStandardPort registry key:", err)
		}
	}

	if insecure {
		err = k.SetStringValue("Insecure", "true")
		if err != nil {
			log.Fatalln("Error creating Insecure registry key:", err)
		}
	}
}

func (a *Agent) checkExistingAndRemove(silent bool) {
	hasReg := false
	_, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\TacticalRMM`, registry.ALL_ACCESS)
	if err == nil {
		hasReg = true
	}
	if hasReg {
		tacUninst := filepath.Join(a.ProgramDir, a.GetUninstallExe())
		tacUninstArgs := [2]string{tacUninst, "/VERYSILENT"}

		window := w32.GetForegroundWindow()
		if !silent && window != 0 {
			var handle w32.HWND
			msg := "Existing installation found\nClick OK to remove, then re-run the installer.\nClick Cancel to abort."
			action := w32.MessageBox(handle, msg, "Tactical RMM", w32.MB_OKCANCEL|w32.MB_ICONWARNING)
			if action == w32.IDOK {
				a.AgentUninstall("foo")
			}
		} else {
			fmt.Println("Existing installation found and must be removed before attempting to reinstall.")
			fmt.Println("Run the following command to uninstall, and then re-run this installer.")
			fmt.Printf(`"%s" %s `, tacUninstArgs[0], tacUninstArgs[1])
		}
		os.Exit(0)
	}
}

func (a *Agent) installerMsg(msg, alert string, silent bool) {
	window := w32.GetForegroundWindow()
	if !silent && window != 0 {
		var (
			handle w32.HWND
			flags  uint
		)

		switch alert {
		case "info":
			flags = w32.MB_OK | w32.MB_ICONINFORMATION
		case "error":
			flags = w32.MB_OK | w32.MB_ICONERROR
		default:
			flags = w32.MB_OK | w32.MB_ICONINFORMATION
		}

		w32.MessageBox(handle, msg, "Tactical RMM", flags)
	} else {
		fmt.Println(msg)
	}

	if alert == "error" {
		a.Logger.Fatalln(msg)
	}
}
