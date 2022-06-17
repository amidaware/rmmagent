package service

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/amidaware/rmmagent/agent/disk"
	"github.com/amidaware/rmmagent/agent/services"
	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/tactical"
	"github.com/amidaware/rmmagent/agent/tactical/checks"
	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/amidaware/rmmagent/agent/wmi"
	"github.com/nats-io/nats.go"
	"github.com/ugorji/go/codec"
	trmm "github.com/wh1te909/trmm-shared"
)

var natsCheckin = []string{"agent-hello", "agent-agentinfo", "agent-disks", "agent-winsvc", "agent-publicip", "agent-wmi"}

func RunAsService(agentID string, version string) {
	var wg sync.WaitGroup
	wg.Add(1)
	go AgentSvc(version)
	go checks.CheckRunner(agentID)
	wg.Wait()
}

func AgentSvc(version string) {
	config := tactical.NewAgentConfig()
	go tactical.GetPython(false)
	utils.CreateTRMMTempDir()
	tactical.RunMigrations()

	sleepDelay := utils.RandRange(14, 22)
	//a.Logger.Debugf("AgentSvc() sleeping for %v seconds", sleepDelay)
	time.Sleep(time.Duration(sleepDelay) * time.Second)

	opts := SetupNatsOptions(config.AgentID, config.Token)
	server := fmt.Sprintf("tls://%s:4222", config.APIURL)
	nc, err := nats.Connect(server, opts...)
	if err != nil {
		//a.Logger.Fatalln("AgentSvc() nats.Connect()", err)
	}

	for _, s := range natsCheckin {
		NatsMessage(config.AgentID, version, nc, s)
		time.Sleep(time.Duration(utils.RandRange(100, 400)) * time.Millisecond)
	}

	go tactical.SyncMeshNodeID()

	time.Sleep(time.Duration(utils.RandRange(1, 3)) * time.Second)
	AgentStartup(config.AgentID)
	tactical.SendSoftware()

	checkInHelloTicker := time.NewTicker(time.Duration(utils.RandRange(30, 60)) * time.Second)
	checkInAgentInfoTicker := time.NewTicker(time.Duration(utils.RandRange(200, 400)) * time.Second)
	checkInWinSvcTicker := time.NewTicker(time.Duration(utils.RandRange(2400, 3000)) * time.Second)
	checkInPubIPTicker := time.NewTicker(time.Duration(utils.RandRange(300, 500)) * time.Second)
	checkInDisksTicker := time.NewTicker(time.Duration(utils.RandRange(1000, 2000)) * time.Second)
	checkInSWTicker := time.NewTicker(time.Duration(utils.RandRange(2800, 3500)) * time.Second)
	checkInWMITicker := time.NewTicker(time.Duration(utils.RandRange(3000, 4000)) * time.Second)
	syncMeshTicker := time.NewTicker(time.Duration(utils.RandRange(800, 1200)) * time.Second)

	for {
		select {
		case <-checkInHelloTicker.C:
			NatsMessage(config.AgentID, version, nc, "agent-hello")
		case <-checkInAgentInfoTicker.C:
			NatsMessage(config.AgentID, version, nc, "agent-agentinfo")
		case <-checkInWinSvcTicker.C:
			NatsMessage(config.AgentID, version, nc, "agent-winsvc")
		case <-checkInPubIPTicker.C:
			NatsMessage(config.AgentID, version, nc, "agent-publicip")
		case <-checkInDisksTicker.C:
			NatsMessage(config.AgentID, version, nc, "agent-disks")
		case <-checkInSWTicker.C:
			tactical.SendSoftware()
		case <-checkInWMITicker.C:
			NatsMessage(config.AgentID, version, nc, "agent-wmi")
		case <-syncMeshTicker.C:
			tactical.SyncMeshNodeID()
		}
	}
}

func SetupNatsOptions(agentID string, token string) []nats.Option {
	opts := make([]nats.Option, 0)
	opts = append(opts, nats.Name("TacticalRMM"))
	opts = append(opts, nats.UserInfo(agentID, token))
	opts = append(opts, nats.ReconnectWait(time.Second*5))
	opts = append(opts, nats.RetryOnFailedConnect(true))
	opts = append(opts, nats.MaxReconnects(-1))
	opts = append(opts, nats.ReconnectBufSize(-1))
	return opts
}

func NatsMessage(agentID string, version string, nc *nats.Conn, mode string) {
	var resp []byte
	var payload interface{}
	ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

	switch mode {
	case "agent-hello":
		payload = trmm.CheckInNats{
			Agentid: agentID,
			Version: version,
		}
	case "agent-winsvc":
		payload = trmm.WinSvcNats{
			Agentid: agentID,
			WinSvcs: services.GetServices(),
		}
	case "agent-agentinfo":
		osinfo := system.OsString()
		reboot, err := system.SystemRebootRequired()
		if err != nil {
			reboot = false
		}
		payload = trmm.AgentInfoNats{
			Agentid:      agentID,
			Username:     system.LoggedOnUser(),
			Hostname:     system.GetHostname(),
			OS:           osinfo,
			Platform:     runtime.GOOS,
			TotalRAM:     system.TotalRAM(),
			BootTime:     system.BootTime(),
			RebootNeeded: reboot,
			GoArch:       runtime.GOARCH,
		}
	case "agent-wmi":
		payload = trmm.WinWMINats{
			Agentid: agentID,
			WMI:     wmi.GetWMIInfo(),
		}
	case "agent-disks":
		payload = trmm.WinDisksNats{
			Agentid: agentID,
			Disks:   disk.GetDisks(),
		}
	case "agent-publicip":
		payload = trmm.PublicIPNats{
			Agentid:  agentID,
			PublicIP: a.PublicIP(),
		}
	}

	//a.Logger.Debugln(mode, payload)
	ret.Encode(payload)
	nc.PublishRequest(a.AgentID, mode, resp)
}

func DoNatsCheckIn() {
	opts := SetupNatsOptions()
	server := fmt.Sprintf("tls://%s:4222", a.ApiURL)
	nc, err := nats.Connect(server, opts...)
	if err != nil {
		//a.Logger.Errorln(err)
		return
	}

	for _, s := range natsCheckin {
		time.Sleep(time.Duration(utils.RandRange(100, 400)) * time.Millisecond)
		NatsMessage(nc, s)
	}
	nc.Close()
}

func AgentStartup(agentID string) {
	url := "/api/v3/checkin/"
	payload := map[string]interface{}{"agent_id": agentID}
	_, err := tactical.PostRequest(url, payload, 15)
	if err != nil {
		//a.Logger.Debugln("AgentStartup()", err)
	}
}
