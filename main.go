/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	"github.com/amidaware/rmmagent/agent"
	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
)

var (
	version = "2.6.2"
	log     = logrus.New()
	logFile *os.File
)

func main() {
	ver := flag.Bool("version", false, "Prints version")
	mode := flag.String("m", "", "The mode to run")
	taskPK := flag.Int("p", 0, "Task PK")
	logLevel := flag.String("log", "INFO", "The log level")
	logTo := flag.String("logto", "file", "Where to log to")
	api := flag.String("api", "", "API URL")
	clientID := flag.Int("client-id", 0, "Client ID")
	siteID := flag.Int("site-id", 0, "Site ID")
	timeout := flag.Duration("timeout", 900, "Installer timeout (seconds)")
	desc := flag.String("desc", "", "Agent's Description")
	atype := flag.String("agent-type", "server", "server or workstation")
	token := flag.String("auth", "", "Token")
	power := flag.Bool("power", false, "Disable sleep/hibernate")
	rdp := flag.Bool("rdp", false, "Enable RDP")
	ping := flag.Bool("ping", false, "Enable ping")
	localMesh := flag.String("local-mesh", "", "Path to mesh executable")
	noMesh := flag.Bool("nomesh", false, "Do not install mesh agent")
	meshDir := flag.String("meshdir", "", "Path to custom meshcentral dir")
	meshNodeID := flag.String("meshnodeid", "", "Mesh Node ID")
	cert := flag.String("cert", "", "Path to domain CA .pem")
	silent := flag.Bool("silent", false, "Do not popup any message boxes during installation")
	proxy := flag.String("proxy", "", "Use a http proxy")
	insecure := flag.Bool("insecure", false, "Insecure for testing only")
	natsport := flag.String("natsport", "", "nats standard port")
	flag.Parse()

	if *ver {
		agent.ShowVersionInfo(version)
		return
	}

	if len(os.Args) == 1 {
		switch runtime.GOOS {
		case "windows":
			agent.ShowStatus(version)
		default:
			agent.ShowVersionInfo(version)
		}
		return
	}

	setupLogging(logLevel, logTo)
	defer logFile.Close()

	a := *agent.New(log, version)

	if *mode == "install" {
		a.Logger.SetOutput(os.Stdout)
	}

	a.Logger.Debugf("%+v\n", a)

	switch *mode {
	case "getenv":
		fmt.Println(os.Getenv(flag.Arg(0)))
	case "nixmeshnodeid":
		fmt.Print(a.NixMeshNodeID())
	case "installsvc":
		a.InstallService()
	case "checkin":
		a.DoNatsCheckIn()
	case "rpc":
		a.RunRPC()
	case "svc":
		if runtime.GOOS == "windows" {
			s, _ := service.New(&a, a.ServiceConfig)
			s.Run()
		} else {
			a.RunRPC()
		}
	case "pk":
		fmt.Println(a.AgentPK)
	case "winagentsvc":
		fmt.Println("deprecated. use 'svc'")
	case "runchecks":
		a.RunChecks(true)
	case "checkrunner":
		a.RunChecks(false)
	case "software":
		a.SendSoftware()
	case "cleanup":
		a.UninstallCleanup()
	case "publicip":
		fmt.Println(a.PublicIP())
	case "getpython":
		a.GetPython(true)
	case "runmigrations":
		a.RunMigrations()
	case "recovermesh":
		a.RecoverMesh()
	case "macventurafix":
		a.FixVenturaMesh()
	case "taskrunner":
		if len(os.Args) < 5 || *taskPK == 0 {
			return
		}
		a.RunTask(*taskPK)
	case "install":
		if runtime.GOOS != "windows" {
			u, err := user.Current()
			if err != nil {
				log.Fatalln(err)
			}
			if u.Uid != "0" {
				log.Fatalln("must run as root")
			}
		}

		if *api == "" || *clientID == 0 || *siteID == 0 || *token == "" {
			return
		}
		a.Install(&agent.Installer{
			RMM:              *api,
			ClientID:         *clientID,
			SiteID:           *siteID,
			Description:      *desc,
			AgentType:        *atype,
			Power:            *power,
			RDP:              *rdp,
			Ping:             *ping,
			Token:            *token,
			LocalMesh:        *localMesh,
			Cert:             *cert,
			Proxy:            *proxy,
			Timeout:          *timeout,
			Silent:           *silent,
			NoMesh:           *noMesh,
			MeshDir:          *meshDir,
			MeshNodeID:       *meshNodeID,
			Insecure:         *insecure,
			NatsStandardPort: *natsport,
		})
	default:
		agent.ShowStatus(version)
	}
}

func setupLogging(level, to *string) {
	ll, err := logrus.ParseLevel(*level)
	if err != nil {
		ll = logrus.InfoLevel
	}
	log.SetLevel(ll)

	if *to == "stdout" {
		log.SetOutput(os.Stdout)
	} else {
		switch runtime.GOOS {
		case "windows":
			logFile, _ = os.OpenFile(filepath.Join(os.Getenv("ProgramFiles"), "TacticalAgent", "agent.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
		default:
			logFile, _ = os.OpenFile(filepath.Join("/var/log/", "tacticalagent.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0664)
		}
		log.SetOutput(logFile)
	}
}
