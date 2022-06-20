package software

import (
	trmm "github.com/wh1te909/trmm-shared"
)

func GetInstalledSoftware() []SoftwareList { return []WinSoftwareList{} }

func InstallChoco() {}

func InstallWithChoco(name string) (string, error) { return "", nil }