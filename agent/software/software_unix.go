//go:build !windows
// +build !windows

package software

func GetInstalledSoftware() ([]Software, error) {
	return []Software{}, nil
}

func InstallChoco() {}

func InstallWithChoco(name string) (string, error) { return "", nil }