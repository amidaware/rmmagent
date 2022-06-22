//go:build !windows
// +build !windows

package config

import (
	"strconv"

	"github.com/spf13/viper"
)

func NewAgentConfig() *AgentConfig {
	viper.SetConfigName("tacticalagent")
	viper.SetConfigType("json")
	viper.AddConfigPath("/etc/")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()

	if err != nil {
		return &AgentConfig{}
	}

	agentpk := viper.GetString("agentpk")
	pk, _ := strconv.Atoi(agentpk)

	ret := &AgentConfig{
		BaseURL:       viper.GetString("baseurl"),
		AgentID:       viper.GetString("agentid"),
		APIURL:        viper.GetString("apiurl"),
		Token:         viper.GetString("token"),
		AgentPK:       agentpk,
		PK:            pk,
		Cert:          viper.GetString("cert"),
		Proxy:         viper.GetString("proxy"),
		CustomMeshDir: viper.GetString("meshdir"),
	}
	return ret
}