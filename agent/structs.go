package agent

import (
	"os"

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
}