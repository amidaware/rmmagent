package agent

import (
	"os"
	"time"

	gocmd "github.com/go-cmd/cmd"
	"github.com/go-resty/resty/v2"
	"github.com/kardianos/service"
	"github.com/sirupsen/logrus"
)

// Agent struct
type Agent struct {
	Hostname      string
	Arch          string
	AgentID       string
	BaseURL       string
	ApiURL        string
	Token         string
	AgentPK       int
	Cert          string
	ProgramDir    string
	EXE           string
	SystemDrive   string
	MeshInstaller string
	MeshSystemBin string
	MeshSVC       string
	PyBin         string
	Headers       map[string]string
	Logger        *logrus.Logger
	Version       string
	Debug         bool
	rClient       *resty.Client
	Proxy         string
	LogTo         string
	LogFile       *os.File
	Platform      string
	GoArch        string
	ServiceConfig *service.Config
	NatsServer    string
	NatsProxyPath string
	NatsProxyPort string
}

type AgentConfig struct {
	BaseURL       string
	AgentID       string
	APIURL        string
	Token         string
	AgentPK       string
	PK            int
	Cert          string
	Proxy         string
	CustomMeshDir string
}

type CmdStatus struct {
	Status gocmd.Status
	Stdout string
	Stderr string
}

type CmdOptions struct {
	Shell        string
	Command      string
	Args         []string
	Timeout      time.Duration
	IsScript     bool
	IsExecutable bool
	Detached     bool
}
