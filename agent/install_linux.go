/*
Copyright 2022 AmidaWare LLC.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"log"

	"github.com/spf13/viper"
)

const (
	etcConfig = "/etc/tacticalagent"
)

func (a *Agent) checkExistingAndRemove(silent bool) {}

func (a *Agent) installerMsg(msg, alert string, silent bool) {
	if alert == "error" {
		a.Logger.Fatalln(msg)
	} else {
		a.Logger.Info(msg)
	}
}

func createAgentConfig(baseurl, agentid, apiurl, token, agentpk, cert, proxy, meshdir string) {
	viper.SetConfigType("json")
	viper.Set("baseurl", baseurl)
	viper.Set("agentid", agentid)
	viper.Set("apiurl", apiurl)
	viper.Set("token", token)
	viper.Set("agentpk", agentpk)
	viper.Set("cert", cert)
	viper.Set("proxy", proxy)
	viper.Set("meshdir", meshdir)
	viper.SetConfigPermissions(0660)
	err := viper.SafeWriteConfigAs(etcConfig)
	if err != nil {
		log.Fatalln("createAgentConfig", err)
	}
}

func (a *Agent) addDefenderExlusions() {}

func DisableSleepHibernate() {}

func EnablePing() {}

func EnableRDP() {}
