//go:build windows

package agent

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func EnsureWinPTY(force bool) (string, error) {
	const progFilesName = "TacticalAgent"
	targetDir := filepath.Join(os.Getenv("ProgramFiles"), progFilesName)
	arch := runtime.GOARCH
	if arch != "amd64" && arch != "386" {
		return "", errors.New("EnsureWinPTY(): unsupported arch: " + arch)
	}

	// Ensure target directory exists
	if stat, err := os.Stat(targetDir); err != nil || !stat.IsDir() {
		return "", fmt.Errorf("expected install directory not found: %s", targetDir)
	}

	dstDLL := filepath.Join(targetDir, "winpty.dll")
	dstAgent := filepath.Join(targetDir, "winpty-agent.exe")

	// Fast path: already present
	if !force && fileExists(dstDLL) && fileExists(dstAgent) {
		return targetDir, nil
	}

	// Read embedded files
	dllEmbedPath := fmt.Sprintf("winpty_bins/%s/winpty.dll", arch)
	agentEmbedPath := fmt.Sprintf("winpty_bins/%s/winpty-agent.exe", arch)

	dllBytes, err := winptyFS.ReadFile(dllEmbedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded DLL: %w", err)
	}

	agentBytes, err := winptyFS.ReadFile(agentEmbedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded EXE: %w", err)
	}

	// Skip rewrite if identical
	if !force &&
		sameContent(dstDLL, dllBytes) &&
		sameContent(dstAgent, agentBytes) {
		return targetDir, nil
	}

	// Write files atomically
	if err := writeAtomic(dstDLL, dllBytes, 0o644); err != nil {
		return "", fmt.Errorf("failed to write DLL: %w", err)
	}

	if err := writeAtomic(dstAgent, agentBytes, 0o755); err != nil {
		return "", fmt.Errorf("failed to write agent: %w", err)
	}

	return targetDir, nil
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func sameContent(path string, want []byte) bool {
	have, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return sha256.Sum256(have) == sha256.Sum256(want)
}

func writeAtomic(dst string, content []byte, perm os.FileMode) error {
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, content, perm); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
