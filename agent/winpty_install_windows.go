//go:build windows

package agent

import (
	"crypto/sha256"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

func EnsureWinPTY(force bool) (string, error) {
	const progFilesName = "TacticalAgent"
	pd := filepath.Join(os.Getenv("ProgramFiles"), progFilesName)
	targetDir := pd

	arch := runtime.GOARCH
	if arch != "amd64" && arch != "386" {
		return "", errors.New("EnsureWinPTY(): unsupported arch: " + arch)
	}

	dstDLL := filepath.Join(targetDir, "winpty.dll")
	dstAgent := filepath.Join(targetDir, "winpty-agent.exe")

	// Fast path: already present
	if !force && fileExists(dstDLL) && fileExists(dstAgent) {
		return targetDir, nil
	}

	// Read embedded files
	dllEmbed := filepath.ToSlash(filepath.Join("build", "winpty_bins", arch, "winpty.dll"))
	agentEmbed := filepath.ToSlash(filepath.Join("build", "winpty_bins", arch, "winpty-agent.exe"))

	dllBytes, err := fs.ReadFile(winptyFS, dllEmbed)
	if err != nil {
		return "", err
	}
	agentBytes, err := fs.ReadFile(winptyFS, agentEmbed)
	if err != nil {
		return "", err
	}

	// Skip rewrite if identical
	if !force &&
		sameContent(dstDLL, dllBytes) &&
		sameContent(dstAgent, agentBytes) {
		return targetDir, nil
	}

	// Atomic write
	if err := writeAtomic(dstDLL, dllBytes, 0o644); err != nil {
		return "", err
	}
	if err := writeAtomic(dstAgent, agentBytes, 0o755); err != nil {
		return "", err
	}

	return targetDir, nil
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func sameContent(path string, want []byte) bool {
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return sha256.Sum256(b) == sha256.Sum256(want)
}

func writeAtomic(dst string, content []byte, perm os.FileMode) error {
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, content, perm); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
