package agent

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"testing"
)

func TestInstall(t *testing.T) {
	testTable := []struct {
		name          string
		expectedError error
		version       string
		log           logrus.Logger
	}{
		{
			name:          "Install",
			expectedError: nil,
			version:       "2.1.0-dev",
			log:           *logrus.New(),
		},
		{
			name:          "Install Error",
			expectedError: nil,
			version:       "bad ver",
			log:           *logrus.New(),
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			a := New(&tt.log, tt.version)

			viper.SetConfigName("testargs.json")
			viper.SetConfigType("json")
			viper.AddConfigPath(".")
			viper.ReadInConfig()

			installer := Installer{
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
		})
	}
}
