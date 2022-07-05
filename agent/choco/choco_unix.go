//go:build !windows
// +build !windows

package choco

//stubbed out for rpc
func InstallChoco() error {
	return nil
}
func InstallWithChoco(name string) (string, error) {
	return "", nil
}
