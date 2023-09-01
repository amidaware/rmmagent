/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"fmt"
	"time"

	rmm "github.com/amidaware/rmmagent/shared"
)

func (a *Agent) GetWinUpdates() {
	updates, err := WUAUpdates("IsInstalled=1 or IsInstalled=0 and Type='Software' and IsHidden=0")
	if err != nil {
		a.Logger.Errorln(err)
		return
	}

	for _, update := range updates {
		a.Logger.Debugln("GUID:", update.UpdateID)
		a.Logger.Debugln("Downloaded:", update.Downloaded)
		a.Logger.Debugln("Installed:", update.Installed)
		a.Logger.Debugln("KB:", update.KBArticleIDs)
		a.Logger.Debugln("--------------------------------")
	}

	payload := rmm.WinUpdateResult{AgentID: a.AgentID, Updates: updates}
	_, err = a.rClient.R().SetBody(payload).Post("/api/v3/winupdates/")
	if err != nil {
		a.Logger.Debugln(err)
	}
}

func (a *Agent) InstallUpdates(guids []string) {
	session, err := NewUpdateSession()
	if err != nil {
		a.Logger.Errorln(err)
		return
	}
	defer session.Close()

	for _, id := range guids {
		var result rmm.WinUpdateInstallResult
		result.AgentID = a.AgentID
		result.UpdateID = id

		query := fmt.Sprintf("UpdateID='%s'", id)
		a.Logger.Debugln("query:", query)
		updts, err := session.GetWUAUpdateCollection(query)
		if err != nil {
			a.Logger.Errorln(err)
			result.Success = false
			a.rClient.R().SetBody(result).Patch("/api/v3/winupdates/")
			continue
		}
		defer updts.Release()

		updtCnt, err := updts.Count()
		if err != nil {
			a.Logger.Errorln(err)
			result.Success = false
			a.rClient.R().SetBody(result).Patch("/api/v3/winupdates/")
			continue
		}
		a.Logger.Debugln("updtCnt:", updtCnt)

		if updtCnt == 0 {
			superseded := rmm.SupersededUpdate{AgentID: a.AgentID, UpdateID: id}
			a.rClient.R().SetBody(superseded).Post("/api/v3/superseded/")
			continue
		}

		for i := 0; i < int(updtCnt); i++ {
			u, err := updts.Item(i)
			if err != nil {
				a.Logger.Errorln(err)
				result.Success = false
				a.rClient.R().SetBody(result).Patch("/api/v3/winupdates/")
				continue
			}
			a.Logger.Debugln("u:", u)
			err = session.InstallWUAUpdate(u)
			if err != nil {
				a.Logger.Errorln(err)
				result.Success = false
				a.rClient.R().SetBody(result).Patch("/api/v3/winupdates/")
				continue
			}
			result.Success = true
			a.rClient.R().SetBody(result).Patch("/api/v3/winupdates/")
			a.Logger.Debugln("Installed windows update with guid", id)
		}
	}

	time.Sleep(5 * time.Second)
	needsReboot, err := a.SystemRebootRequired()
	if err != nil {
		a.Logger.Errorln(err)
	}
	rebootPayload := rmm.AgentNeedsReboot{AgentID: a.AgentID, NeedsReboot: needsReboot}
	_, err = a.rClient.R().SetBody(rebootPayload).Put("/api/v3/winupdates/")
	if err != nil {
		a.Logger.Debugln("NeedsReboot:", err)
	}
}
