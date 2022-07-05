//go:build !windows
// +build !windows

package install

import (
	"github.com/spf13/viper"
	"log"
)

func CheckExistingAndRemove(silent bool) error {
	return nil
}

func CreateAgentConfig(baseurl, agentid, apiurl, token, agentpk, cert, proxy, meshdir string) {
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
	configLocation := "/etc/tacticalagent"

	err := viper.SafeWriteConfigAs(configLocation)

	if err != nil {
		log.Fatalln("createAgentConfig", err)
	}
}

func DisableSleepHibernate() {}

func EnablePing() {}

func EnableRDP() {}
