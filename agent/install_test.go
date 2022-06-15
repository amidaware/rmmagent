package agent

import (
	"testing"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"strconv"
)

func TestInstall(t *testing.T) {
	var (
		version = "2.0.4"
		log     = logrus.New()
	)

	a := New(log, version)

	viper.SetConfigName("testargs.json")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")

	cid, err := strconv.Atoi(viper.GetString("clientid"))

	if err != nil {
		cid = 0
	}

	installer := Installer {
		RMM:         viper.GetString("api"),
		ClientID:    cid,
		SiteID:      *siteID,
		Description: *desc,
		AgentType:   *atype,
		Power:       *power,
		RDP:         *rdp,
		Ping:        *ping,
		Token:       *token,
		LocalMesh:   *localMesh,
		Cert:        *cert,
		Proxy:       *proxy,
		Timeout:     *timeout,
		Silent:      *silent,
		NoMesh:      *noMesh,
		MeshDir:     *meshDir,
		MeshNodeID:  *meshNodeID,
	}

	a.Install(&installer)
}
