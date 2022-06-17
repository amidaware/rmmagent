/*
Copyright 2022 AmidaWare LLC.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	rmm "github.com/amidaware/rmmagent/shared"
	ps "github.com/elastic/go-sysinfo"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/go-resty/resty/v2"
	"github.com/gonutz/w32/v2"
	"github.com/kardianos/service"
	"github.com/shirou/gopsutil/v3/disk"
	wapf "github.com/wh1te909/go-win64api"
	trmm "github.com/wh1te909/trmm-shared"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)











func (a *Agent) RecoverMesh() {
	a.Logger.Infoln("Attempting mesh recovery")
	defer CMD("net", []string{"start", a.MeshSVC}, 60, false)

	_, _ = CMD("net", []string{"stop", a.MeshSVC}, 60, false)
	a.ForceKillMesh()
	a.SyncMeshNodeID()
}





func (a *Agent) Stop(_ service.Service) error {
	return nil
}

func (a *Agent) InstallService() error {
	if serviceExists(winSvcName) {
		return nil
	}

	// skip on first call of inno setup if this is a new install
	_, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\TacticalRMM`, registry.ALL_ACCESS)
	if err != nil {
		return nil
	}

	s, err := service.New(a, a.ServiceConfig)
	if err != nil {
		return err
	}

	return service.Control(s, "install")
}

// TODO add to stub
func (a *Agent) NixMeshNodeID() string {
	return "not implemented"
}
