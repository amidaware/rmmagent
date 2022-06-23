package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/amidaware/rmmagent/agent/tactical/api"
	"github.com/amidaware/rmmagent/agent/tactical/checks"
	"github.com/amidaware/rmmagent/agent/tactical/config"
	"github.com/amidaware/rmmagent/agent/tactical/mesh"
	"github.com/amidaware/rmmagent/agent/tactical/shared"
	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/nats-io/nats.go"
)

var natsCheckin = []string{"agent-hello", "agent-agentinfo", "agent-disks", "agent-winsvc", "agent-publicip", "agent-wmi"}

func RunAsService(version string) {
	var wg sync.WaitGroup
	wg.Add(1)
	go AgentSvc(version)
	go checks.CheckRunner(version)
	wg.Wait()
}

func AgentSvc(version string) error {
	config := config.NewAgentConfig()
	go shared.GetPython(false)
	utils.CreateTRMMTempDir()
	shared.RunMigrations()
	sleepDelay := utils.RandRange(14, 22)
	time.Sleep(time.Duration(sleepDelay) * time.Second)
	opts := SetupNatsOptions()
	server := fmt.Sprintf("tls://%s:4222", config.APIURL)
	nc, err := nats.Connect(server, opts...)
	if err != nil {
		return err
	}

	for _, s := range natsCheckin {
		NatsMessage(version, nc, s)
		time.Sleep(time.Duration(utils.RandRange(100, 400)) * time.Millisecond)
	}

	go mesh.SyncMeshNodeID()

	time.Sleep(time.Duration(utils.RandRange(1, 3)) * time.Second)
	AgentStartup(config.AgentID)
	shared.SendSoftware()

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
			NatsMessage(version, nc, "agent-hello")
		case <-checkInAgentInfoTicker.C:
			NatsMessage(version, nc, "agent-agentinfo")
		case <-checkInWinSvcTicker.C:
			NatsMessage(version, nc, "agent-winsvc")
		case <-checkInPubIPTicker.C:
			NatsMessage(version, nc, "agent-publicip")
		case <-checkInDisksTicker.C:
			NatsMessage(version, nc, "agent-disks")
		case <-checkInSWTicker.C:
			shared.SendSoftware()
		case <-checkInWMITicker.C:
			NatsMessage(version, nc, "agent-wmi")
		case <-syncMeshTicker.C:
			mesh.SyncMeshNodeID()
		}
	}
}

func SetupNatsOptions() []nats.Option {
	config := config.NewAgentConfig()
	opts := make([]nats.Option, 0)
	opts = append(opts, nats.Name("TacticalRMM"))
	opts = append(opts, nats.UserInfo(config.AgentID, config.Token))
	opts = append(opts, nats.ReconnectWait(time.Second*5))
	opts = append(opts, nats.RetryOnFailedConnect(true))
	opts = append(opts, nats.MaxReconnects(-1))
	opts = append(opts, nats.ReconnectBufSize(-1))
	return opts
}

func DoNatsCheckIn(version string) {
	opts := SetupNatsOptions()
	server := fmt.Sprintf("tls://%s:4222", config.NewAgentConfig().APIURL)
	nc, err := nats.Connect(server, opts...)
	if err != nil {
		return
	}

	for _, s := range natsCheckin {
		time.Sleep(time.Duration(utils.RandRange(100, 400)) * time.Millisecond)
		NatsMessage(version, nc, s)
	}

	nc.Close()
}

func AgentStartup(agentID string) error {
	payload := map[string]interface{}{"agent_id": agentID}
	err := api.PostPayload(payload, "/api/v3/checkin/")
	return err
}
