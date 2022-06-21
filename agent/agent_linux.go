package agent

func New(logger *logrus.Logger, version string) *Agent {
	host, _ := ps.Host()
	info := host.Info()
	pd := filepath.Join(os.Getenv("ProgramFiles"), progFilesName)
	exe := filepath.Join(pd, winExeName)
	sd := os.Getenv("SystemDrive")

	var pybin string
	switch runtime.GOARCH {
	case "amd64":
		pybin = filepath.Join(pd, "py38-x64", "python.exe")
	case "386":
		pybin = filepath.Join(pd, "py38-x32", "python.exe")
	}

	ac := NewAgentConfig()

	headers := make(map[string]string)
	if len(ac.Token) > 0 {
		headers["Content-Type"] = "application/json"
		headers["Authorization"] = fmt.Sprintf("Token %s", ac.Token)
	}

	restyC := resty.New()
	restyC.SetBaseURL(ac.BaseURL)
	restyC.SetCloseConnection(true)
	restyC.SetHeaders(headers)
	restyC.SetTimeout(15 * time.Second)
	restyC.SetDebug(logger.IsLevelEnabled(logrus.DebugLevel))

	if len(ac.Proxy) > 0 {
		restyC.SetProxy(ac.Proxy)
	}

	if len(ac.Cert) > 0 {
		restyC.SetRootCertificate(ac.Cert)
	}

	var MeshSysBin string
	if len(ac.CustomMeshDir) > 0 {
		MeshSysBin = filepath.Join(ac.CustomMeshDir, "MeshAgent.exe")
	} else {
		MeshSysBin = filepath.Join(os.Getenv("ProgramFiles"), "Mesh Agent", "MeshAgent.exe")
	}

	if runtime.GOOS == "linux" {
		MeshSysBin = "/opt/tacticalmesh/meshagent"
	}

	svcConf := &service.Config{
		Executable:  exe,
		Name:        winSvcName,
		DisplayName: "TacticalRMM Agent Service",
		Arguments:   []string{"-m", "svc"},
		Description: "TacticalRMM Agent Service",
		Option: service.KeyValue{
			"StartType":              "automatic",
			"OnFailure":              "restart",
			"OnFailureDelayDuration": "5s",
			"OnFailureResetPeriod":   10,
		},
	}

	return &Agent{
		Hostname:      info.Hostname,
		Arch:          info.Architecture,
		BaseURL:       ac.BaseURL,
		AgentID:       ac.AgentID,
		ApiURL:        ac.APIURL,
		Token:         ac.Token,
		AgentPK:       ac.PK,
		Cert:          ac.Cert,
		ProgramDir:    pd,
		EXE:           exe,
		SystemDrive:   sd,
		MeshInstaller: "meshagent.exe",
		MeshSystemBin: MeshSysBin,
		MeshSVC:       meshSvcName,
		PyBin:         pybin,
		Headers:       headers,
		Logger:        logger,
		Version:       version,
		Debug:         logger.IsLevelEnabled(logrus.DebugLevel),
		rClient:       restyC,
		Proxy:         ac.Proxy,
		Platform:      runtime.GOOS,
		GoArch:        runtime.GOARCH,
		ServiceConfig: svcConf,
	}
}