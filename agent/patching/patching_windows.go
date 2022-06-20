package patching

import (
	"github.com/amidaware/rmmagent/agent/patching/wua"
	"golang.org/x/sys/windows/registry"
)

// PatchMgmnt enables/disables automatic update
// 0 - Enable Automatic Updates (Default)
// 1 - Disable Automatic Updates
// https://docs.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2008-R2-and-2008/dd939844(v=ws.10)?redirectedfrom=MSDN
func PatchMgmnt(enable bool) error {
	var val uint32
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU`, registry.ALL_ACCESS)
	if err != nil {
		return err
	}

	if enable {
		val = 1
	} else {
		val = 0
	}

	err = k.SetDWordValue("AUOptions", val)
	if err != nil {
		return err
	}

	return nil
}

type PackageList []Package

func GetUpdates() (PackageList, error) {
	wuaupdates, err := wua.WUAUpdates("IsInstalled=1 or IsInstalled=0 and Type='Software' and IsHidden=0")
	packages := []Package{}
	for _, p := range wuaupdates {
		packages = append(packages, Package(p))
	}

	return packages, err
	// if err != nil {
	// 	a.Logger.Errorln(err)
	// 	return
	// }

	// for _, update := range updates {
	// 	a.Logger.Debugln("GUID:", update.UpdateID)
	// 	a.Logger.Debugln("Downloaded:", update.Downloaded)
	// 	a.Logger.Debugln("Installed:", update.Installed)
	// 	a.Logger.Debugln("KB:", update.KBArticleIDs)
	// 	a.Logger.Debugln("--------------------------------")
	// }

	// payload := rmm.WinUpdateResult{AgentID: a.AgentID, Updates: updates}
	// _, err = a.rClient.R().SetBody(payload).Post("/api/v3/winupdates/")
	// if err != nil {
	// 	a.Logger.Debugln(err)
	// }
}
