package patching

import "golang.org/x/sys/windows/registry"

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