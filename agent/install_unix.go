//go:build !windows
// +build !windows

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
	"runtime"
	"time"

	"github.com/spf13/viper"
	trmm "github.com/wh1te909/trmm-shared"
)

func (a *Agent) installerMsg(msg, alert string, silent bool) {
	if alert == "error" {
		a.Logger.Fatalln(msg)
	} else {
		a.Logger.Info(msg)
	}
}

func createAgentConfig(baseurl, agentid, apiurl, token, agentpk, cert, proxy, meshdir, natsport string, insecure bool) {
	viper.SetConfigType("json")
	viper.Set("baseurl", baseurl)
	viper.Set("agentid", agentid)
	viper.Set("apiurl", apiurl)
	viper.Set("token", token)
	viper.Set("agentpk", agentpk)
	viper.Set("cert", cert)
	viper.Set("proxy", proxy)
	viper.Set("meshdir", meshdir)
	viper.Set("natsstandardport", natsport)
	if insecure {
		viper.Set("insecure", "true")
	}
	viper.SetConfigPermissions(0660)
	err := viper.SafeWriteConfigAs(etcConfig)
	if err != nil {
		log.Fatalln("createAgentConfig", err)
	}
}

func (a *Agent) checkExistingAndRemove(silent bool) {
	if runtime.GOOS == "darwin" {
		if trmm.FileExists(a.MeshSystemEXE) {
			a.Logger.Infoln("Existing meshagent found, attempting to remove...")
			uopts := a.NewCMDOpts()
			uopts.Command = fmt.Sprintf("%s -fulluninstall", a.MeshSystemEXE)
			uout := a.CmdV2(uopts)
			fmt.Println(uout.Stdout)
			time.Sleep(1 * time.Second)
		}

		if trmm.FileExists(macPlistPath) {
			a.Logger.Infoln("Existing tacticalagent plist found, attempting to remove...")
			opts := a.NewCMDOpts()
			opts.Command = fmt.Sprintf("launchctl bootout system %s", macPlistPath)
			a.CmdV2(opts)
		}

		os.RemoveAll(defaultMacMeshSvcDir)
		os.RemoveAll(nixMeshDir)
		os.Remove(etcConfig)
		os.RemoveAll(nixAgentDir)
		os.Remove(macPlistPath)
	}
}

func DisableSleepHibernate() {}

func EnablePing() {}

func EnableRDP() {}
