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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	winpty "github.com/iamacarpet/go-winpty"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
)

func startTerminalSessionWinPTY(agentID, programDir, sessionID, shell string, runAsUser bool, nc *nats.Conn, logger *logrus.Logger) error {
	winTermMu.Lock()
	if _, exists := winTerms[sessionID]; exists {
		winTermMu.Unlock()
		return fmt.Errorf("session already exists: %s", sessionID)
	}
	winTermMu.Unlock()

	if runAsUser {
		SendTerminalInfo(
			agentID,
			sessionID,
			"This EOL Windows version does not support remote terminal 'Run as user'. Continuing as SYSTEM.",
			nc,
		)
	}

	prefix, err := EnsureWinPTY(programDir, false)
	if err != nil {
		return fmt.Errorf("EnsureWinPTY() %v", err)
	}

	dll := filepath.Join(prefix, "winpty.dll")
	agentExe := filepath.Join(prefix, "winpty-agent.exe")
	if _, err := os.Stat(dll); err != nil {
		return fmt.Errorf("winpty missing winpty.dll at %s: %w", dll, err)
	}

	if _, err := os.Stat(agentExe); err != nil {
		return fmt.Errorf("winpty missing winpty-agent.exe at %s: %w", agentExe, err)
	}

	cmdline := winptyCommandLine(shell)
	home := resolveWindowsHomeDir()
	if home != "" {
		if st, err := os.Stat(home); err != nil || !st.IsDir() {
			home = ""
		}
	}

	opts := winpty.Options{
		DLLPrefix:   prefix,
		AppName:     "",
		Command:     cmdline,
		Dir:         home,
		Env:         os.Environ(),
		Flags:       0,
		InitialCols: 120,
		InitialRows: 30,
	}

	logger.Debugf(
		"winpty opts: cmd=%q dir=%q prefix=%q flags=%d",
		opts.Command,
		opts.Dir,
		opts.DLLPrefix,
		opts.Flags,
	)

	wp, err := winpty.OpenWithOptions(opts)
	if err != nil {
		return fmt.Errorf(
			"winpty open failed (cmd=%q dir=%q prefix=%q): %+v",
			opts.Command, opts.Dir, opts.DLLPrefix, err,
		)
	}

	cleanup := true
	defer func() {
		if cleanup {
			wp.Close()
		}
	}()

	sess := &winTerminalSession{
		id:      sessionID,
		backend: "winpty",
		wp:      wp,
		wpIn:    wp.StdIn,
		wpOut:   wp.StdOut,
	}

	if ph := wp.GetProcHandle(); ph != 0 {
		sess.proc = windows.Handle(ph)
	}

	// Register only after WinPTY successfully opened
	winTermMu.Lock()
	winTerms[sessionID] = sess
	winTermMu.Unlock()

	applyPendingResizeWindows(sessionID, logger)
	cleanup = false
	go streamTerminalOutputWindows(agentID, sessionID, sess.wpOut, nc, logger)

	go func() {
		if sess.proc != 0 {
			_, _ = windows.WaitForSingleObject(sess.proc, windows.INFINITE)

			var code uint32
			_ = windows.GetExitCodeProcess(sess.proc, &code)

			if StopTerminalSessionWindows(sessionID) {
				sendTerminalDoneWindows(agentID, sessionID, int(code), nc)
			}
			return
		}

		_ = StopTerminalSessionWindows(sessionID)
	}()

	return nil
}

func winptyCommandLine(shell string) string {
	s := strings.ToLower(strings.TrimSpace(shell))

	switch s {
	case "powershell", "powershell.exe":
		ps := getPowershellExe()
		return quoteIfNeeded(ps)

	case "cmd", "cmd.exe", "":
		cmd := getCMDExe()
		return quoteIfNeeded(cmd)

	default:
		cmd := getCMDExe()
		return quoteIfNeeded(cmd)
	}
}

// winptyPrefixDir returns the directory where winpty.dll and winpty-agent.exe are located. ( local go run main.go workaround )
// func winptyPrefixDir() string {
// 	if v := strings.TrimSpace(`C:\Users\Administrator\rmmagent`); v != "" {
// 		return v
// 	}
// 	return "."
// }
