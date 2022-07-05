//go:build !windows
// +build !windows

package patching

func PatchMgmnt(enable bool) error { return nil }

func GetUpdates()(PackageList, error) {
	return PackageList{}, nil
}

func InstallUpdates(guids []string) {}
