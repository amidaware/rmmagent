package software

import (
	trmm "github.com/wh1te909/trmm-shared"
)

func GetInstalledSoftware() []trmm.WinSoftwareList { return []trmm.WinSoftwareList{} }

func InstallChoco() {}

func InstallWithChoco(name string) (string, error) { return "", nil }