package software

import (
	"fmt"

	"github.com/amidaware/rmmagent/agent/utils"
	wapi "github.com/iamacarpet/go-win64api"
	trmm "github.com/wh1te909/trmm-shared"
)

func GetInstalledSoftware() []trmm.WinSoftwareList {
	ret := make([]trmm.WinSoftwareList, 0)

	sw, err := installedSoftwareList()
	if err != nil {
		return ret
	}

	for _, s := range sw {
		t := s.InstallDate
		ret = append(ret, trmm.WinSoftwareList{
			Name:        utils.CleanString(s.Name()),
			Version:     utils.CleanString(s.Version()),
			Publisher:   utils.CleanString(s.Publisher),
			InstallDate: fmt.Sprintf("%02d-%d-%02d", t.Year(), t.Month(), t.Day()),
			Size:        utils.ByteCountSI(s.EstimatedSize * 1024),
			Source:      utils.CleanString(s.InstallSource),
			Location:    utils.CleanString(s.InstallLocation),
			Uninstall:   utils.CleanString(s.UninstallString),
		})
	}
	return ret
}