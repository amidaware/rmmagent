//go:build windows

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// "runtime"

	winpty "github.com/iamacarpet/go-winpty"
	"github.com/nats-io/nats.go"
	"golang.org/x/sys/windows"
)

func startTerminalSessionWinPTY(agentID, sessionID, shell string, nc *nats.Conn) error {
	// Prevent duplicate session IDs
	winTermMu.Lock()
	if _, exists := winTerms[sessionID]; exists {
		winTermMu.Unlock()
		return fmt.Errorf("session already exists: %s", sessionID)
	}
	winTermMu.Unlock()

	prefix, err := EnsureWinPTY(false)
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

	// Keep logs low-noise: don't print env (may contain secrets).
	// If you have an agent logger available in this package, replace this with it.
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

	// Process handle (if exposed by the library version)
	if ph := wp.GetProcHandle(); ph != 0 {
		sess.proc = windows.Handle(ph)
	}

	// Register only after WinPTY successfully opened
	winTermMu.Lock()
	winTerms[sessionID] = sess
	winTermMu.Unlock()

	// ownership transferred to session lifecycle (Stop/Kill should close wp in cleanupWinSession)
	cleanup = false

	// Stream output to NATS (same function as your ConPTY stream path)
	go streamTerminalOutputWindows(agentID, sessionID, sess.wpOut, nc)

	// Exit watcher
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

		// If no proc handle is available, best-effort cleanup.
		_ = StopTerminalSessionWindows(sessionID)
	}()

	return nil
}

func winptyCommandLine(shell string) string {
	s := strings.ToLower(strings.TrimSpace(shell))

	switch s {
	case "powershell", "powershell.exe":
		ps := getPowershellExe()
		// Keep it simple/stable on legacy hosts.
		return quoteIfNeeded(ps)

	case "cmd", "cmd.exe", "":
		cmd := getCMDExe()
		return quoteIfNeeded(cmd)

	default:
		// Strict allowlist fallback.
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
