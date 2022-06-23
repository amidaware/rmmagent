package agent

import (
	"testing"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)



func TestInstall(t *testing.T) {
	testTable := []struct {
		name string
		expectedError error
		version string
	}{
		{
			name: "Install",
			expectedError: nil,
			version: "2.0.4",
		},
		{
			name: "Install Error",
			expectedError: nil,
			version: "bad ver",
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			
		})
	}

	var (
		version = "2.0.4"
		log     = logrus.New()
	)

	a := New(log, version)

	viper.SetConfigName("testargs.json")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")
	viper.ReadInConfig()

	installer := Installer {
		RMM:         viper.GetString("api"),
		ClientID:    viper.GetInt("clientid"),
		SiteID:      viper.GetInt("siteid"),
		Description: viper.GetString("description"),
		AgentType:   viper.GetString("agenttype"),
		Power:       viper.GetBool("power"),
		RDP:         viper.GetBool("rdp"),
		Ping:        viper.GetBool("ping"),
		Token:       viper.GetString("token"),
		LocalMesh:   viper.GetString("localmesh"),
		Cert:        viper.GetString("cert"),
		Proxy:       viper.GetString("proxy"),
		Timeout:     viper.GetDuration("timeout"),
		Silent:      viper.GetBool("silent"),
		NoMesh:      viper.GetBool("nomesh"),
		MeshDir:     viper.GetString("meshdir"),
		MeshNodeID:  viper.GetString("meshnodeid"),
	}

	a.Install(&installer)
}
