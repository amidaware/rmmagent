//go:build freebsd
// +build freebsd

/*
Copyright 2022 AmidaWare LLC.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"
	trmm "github.com/wh1te909/trmm-shared"
)

const (
	etcConfig = "/etc/tacticalagent"
)

func (a *Agent) Install(i *Installer) {
	a.checkExistingAndRemove(i.Silent)

	i.Headers = map[string]string{
		"content-type":  "application/json",
		"Authorization": fmt.Sprintf("Token %s", i.Token),
	}
	a.AgentID = GenerateAgentID()
	a.Logger.Debugln("Agent ID:", a.AgentID)

	u, err := url.Parse(i.RMM)
	if err != nil {
		a.installerMsg(err.Error(), "error", i.Silent)
	}

	if u.Scheme != "https" && u.Scheme != "http" {
		a.installerMsg("Invalid URL (must contain https or http)", "error", i.Silent)
	}

	// will match either ipv4 , or ipv4:port
	var ipPort = regexp.MustCompile(`[0-9]+(?:\.[0-9]+){3}(:[0-9]+)?`)

	// if ipv4:port, strip the port to get ip for salt master
	if ipPort.MatchString(u.Host) && strings.Contains(u.Host, ":") {
		i.SaltMaster = strings.Split(u.Host, ":")[0]
	} else if strings.Contains(u.Host, ":") {
		i.SaltMaster = strings.Split(u.Host, ":")[0]
	} else {
		i.SaltMaster = u.Host
	}

	a.Logger.Debugln("API:", i.SaltMaster)

	terr := TestTCP(fmt.Sprintf("%s:4222", i.SaltMaster))
	if terr != nil {
		a.installerMsg(fmt.Sprintf("ERROR: Either port 4222 TCP is not open on your RMM, or nats.service is not running.\n\n%s", terr.Error()), "error", i.Silent)
	}

	baseURL := u.Scheme + "://" + u.Host
	a.Logger.Debugln("Base URL:", baseURL)

	iClient := resty.New()
	iClient.SetCloseConnection(true)
	iClient.SetTimeout(15 * time.Second)
	iClient.SetDebug(a.Debug)
	iClient.SetHeaders(i.Headers)

	// set proxy if applicable
	if len(i.Proxy) > 0 {
		a.Logger.Infoln("Using proxy:", i.Proxy)
		iClient.SetProxy(i.Proxy)
	}

	creds, cerr := iClient.R().Get(fmt.Sprintf("%s/api/v3/installer/", baseURL))
	if cerr != nil {
		a.installerMsg(cerr.Error(), "error", i.Silent)
	}
	if creds.StatusCode() == 401 {
		a.installerMsg("Installer token has expired. Please generate a new one.", "error", i.Silent)
	}

	verPayload := map[string]string{"version": a.Version}
	iVersion, ierr := iClient.R().SetBody(verPayload).Post(fmt.Sprintf("%s/api/v3/installer/", baseURL))
	if ierr != nil {
		a.installerMsg(ierr.Error(), "error", i.Silent)
	}
	if iVersion.StatusCode() != 200 {
		a.installerMsg(DjangoStringResp(iVersion.String()), "error", i.Silent)
	}

	rClient := resty.New()
	rClient.SetCloseConnection(true)
	rClient.SetTimeout(i.Timeout * time.Second)
	rClient.SetDebug(a.Debug)
	// set rest knox headers
	rClient.SetHeaders(i.Headers)

	// set local cert if applicable
	if len(i.Cert) > 0 {
		if !trmm.FileExists(i.Cert) {
			a.installerMsg(fmt.Sprintf("%s does not exist", i.Cert), "error", i.Silent)
		}
		rClient.SetRootCertificate(i.Cert)
	}

	if len(i.Proxy) > 0 {
		rClient.SetProxy(i.Proxy)
	}

	a.Logger.Infoln("Adding agent to dashboard")
	// add agent
	type NewAgentResp struct {
		AgentPK int    `json:"pk"`
		Token   string `json:"token"`
	}
	agentPayload := map[string]interface{}{
		"agent_id":        a.AgentID,
		"hostname":        a.Hostname,
		"site":            i.SiteID,
		"monitoring_type": i.AgentType,
		"mesh_node_id":    "",
		"description":     i.Description,
		"goarch":          a.GoArch,
		"plat":            a.Platform,
	}

	r, err := rClient.R().SetBody(agentPayload).SetResult(&NewAgentResp{}).Post(fmt.Sprintf("%s/api/v3/newagent/", baseURL))
	if err != nil {
		a.installerMsg(err.Error(), "error", i.Silent)
	}
	if r.StatusCode() != 200 {
		a.installerMsg(r.String(), "error", i.Silent)
	}

	agentPK := r.Result().(*NewAgentResp).AgentPK
	agentToken := r.Result().(*NewAgentResp).Token
	a.Logger.Debugln("Agent token:", agentToken)
	a.Logger.Debugln("Agent PK:", agentPK)
	createAgentConfig(baseURL, a.AgentID, i.SaltMaster, agentToken, strconv.Itoa(agentPK), i.Cert, i.Proxy, i.MeshDir)
	time.Sleep(1 * time.Second)
	// refresh our agent with new values
	a = New(a.Logger, a.Version)
	a.Logger.Debugf("%+v\n", a)
	// set new headers, no longer knox auth...use agent auth
	rClient.SetHeaders(a.Headers)
	time.Sleep(3 * time.Second)
	// check in once
	a.DoNatsCheckIn()
	// send software api
	a.SendSoftware()
	a.Logger.Debugln("Creating temp dir")
	a.CreateTRMMTempDir()
	a.Logger.Infoln("Installing service...")
	err = a.InstallService()
	if err != nil {
		a.installerMsg(err.Error(), "error", i.Silent)
	}

	time.Sleep(1 * time.Second)
	a.Logger.Infoln("Starting service...")
	out := a.ControlService(winSvcName, "start")
	if !out.Success {
		a.installerMsg(out.ErrorMsg, "error", i.Silent)
	}

	a.installerMsg("Installation was successfull!\nAllow a few minutes for the agent to properly display in the RMM", "info", i.Silent)
}

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
