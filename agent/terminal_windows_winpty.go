//go:build windows

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	winpty "github.com/iamacarpet/go-winpty"
	"github.com/nats-io/nats.go"
	"golang.org/x/sys/windows"
)

func startTerminalSessionWinPTY(agentID, programDir, sessionID, shell string, runAsUser bool, nc *nats.Conn) error {
	// Prevent duplicate session IDs
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
			"Run as user is not supported on legacy Windows terminals. Continuing with SYSTEM.",
			nc,
		)
	}

	prefix, err := EnsureWinPTY(programDir, false)
	if err != nil {
		return fmt.Errorf("EnsureWinPTY() %v", err)
	}

	// Validate required WinPTY files exist next to the agent
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

	fmt.Printf("winpty opts => cmd=%q dir=%q prefix=%q flags=%d\n", opts.Command, opts.Dir, opts.DLLPrefix, opts.Flags)

	wp, err := winpty.OpenWithOptions(opts)
	if err != nil {
		return fmt.Errorf(
			"winpty open failed (cmd=%q dir=%q prefix=%q): %+v",
			opts.Command, opts.Dir, opts.DLLPrefix, err,
		)
	}

	// If anything fails after we opened, close WinPTY to avoid leaking winpty-agent.exe
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

	applyPendingResizeWindows(sessionID)
	cleanup = false
	go streamTerminalOutputWindows(agentID, sessionID, sess.wpOut, nc)

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
