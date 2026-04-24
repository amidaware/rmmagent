//go:build windows

/*
Copyright 2025 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func EnsureWinPTY(programDir string, force bool) (string, error) {
	arch := runtime.GOARCH
	if arch != "amd64" && arch != "386" {
		return "", errors.New("EnsureWinPTY(): unsupported arch: " + arch)
	}

	if stat, err := os.Stat(programDir); err != nil || !stat.IsDir() {
		return "", fmt.Errorf("expected install directory not found: %s", programDir)
	}

	dstDLL := filepath.Join(programDir, "winpty.dll")
	dstAgent := filepath.Join(programDir, "winpty-agent.exe")

	if !force && fileExists(dstDLL) && fileExists(dstAgent) {
		return programDir, nil
	}

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

	if !force &&
		sameContent(dstDLL, dllBytes) &&
		sameContent(dstAgent, agentBytes) {
		return programDir, nil
	}

	if err := writeAtomic(dstDLL, dllBytes, 0o644); err != nil {
		return "", fmt.Errorf("failed to write DLL: %w", err)
	}

	if err := writeAtomic(dstAgent, agentBytes, 0o755); err != nil {
		return "", fmt.Errorf("failed to write agent: %w", err)
	}

	return programDir, nil
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
