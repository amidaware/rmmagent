//go:build windows

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/fourcorelabs/wintoken"
	winpty "github.com/iamacarpet/go-winpty"
	"github.com/nats-io/nats.go"
	"golang.org/x/sys/windows"
)

var (
	adapi32                     = windows.NewLazySystemDLL("advapi32.dll")
	procImpersonateLoggedOnUser = adapi32.NewProc("ImpersonateLoggedOnUser")
	procRevertToSelf            = adapi32.NewProc("RevertToSelf")
)

func impersonateLoggedOnUser(token windows.Token) error {
	r1, _, e1 := procImpersonateLoggedOnUser.Call(uintptr(token))
	if r1 == 0 {
		return e1
	}
	return nil
}

func revertToSelf() {
	_, _, _ = procRevertToSelf.Call()
}

// buildUserEnvSlice converts a Windows environment block (*uint16, double-null terminated) into a []string slice compatible with winpty.Options.Env.
// Windows guarantees env vars are <= 32767 UTF-16 chars (MAX_PATH * 2), so the [1 << 15]uint16 cast is safe.
func buildUserEnvSlice(token syscall.Token) ([]string, error) {
	block, err := CreateEnvironmentBlock(token)
	if err != nil {
		return nil, err
	}
	defer DestroyEnvironmentBlock(block)

	var env []string
	p := unsafe.Pointer(block)
	for {
		// Read one null-terminated UTF-16 string
		ptr := (*[1 << 15]uint16)(p)
		var i int
		for i = 0; ptr[i] != 0; i++ {
		}
		if i == 0 {
			break
		}
		env = append(env, windows.UTF16ToString(ptr[:i]))
		p = unsafe.Pointer(uintptr(p) + uintptr((i+1)*2))
	}
	return env, nil
}

// resolveUserHomeDir retrieves the profile directory for the given token.
// Falls back to resolveWindowsHomeDir() (SYSTEM context) on any error.
func resolveUserHomeDir(token windows.Token) string {
	var size uint32
	_ = windows.GetUserProfileDirectory(token, nil, &size)
	if size > 0 {
		buf := make([]uint16, size)
		if err := windows.GetUserProfileDirectory(token, &buf[0], &size); err == nil {
			if dir := windows.UTF16ToString(buf); dir != "" {
				if st, err := os.Stat(dir); err == nil && st.IsDir() {
					return dir
				}
			}
		}
	}
	return resolveWindowsHomeDir()
}

func startTerminalSessionWinPTY(agentID, sessionID, shell string, runAsUser bool, nc *nats.Conn) error {
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

	// Validate required WinPTY files exist
	dll := filepath.Join(prefix, "winpty.dll")
	agentExe := filepath.Join(prefix, "winpty-agent.exe")

	if _, err := os.Stat(dll); err != nil {
		return fmt.Errorf("winpty missing winpty.dll at %s: %w", dll, err)
	}
	if _, err := os.Stat(agentExe); err != nil {
		return fmt.Errorf("winpty missing winpty-agent.exe at %s: %w", agentExe, err)
	}

	cmdline := winptyCommandLine(shell)

	var (
		userToken    *wintoken.Token // nil -> run as SYSTEM
		userEnv      []string        // nil -> use os.Environ()
		home         string
		launchAsUser = false
	)

	if runAsUser {
		userToken, err = getTerminalUserToken()
		if err != nil {
			fmt.Printf("[WARN] winpty user token unavailable for session=%s: %v. Falling back to SYSTEM.\n", sessionID, err)
		} else {
			env, envErr := buildUserEnvSlice(syscall.Token(userToken.Token()))
			if envErr != nil {
				fmt.Printf("[WARN] winpty user env unavailable for session=%s: %v. Falling back to SYSTEM.\n", sessionID, envErr)
				userToken.Close()
				userToken = nil
			} else {
				userEnv = env
				home = resolveUserHomeDir(windows.Token(userToken.Token()))
				launchAsUser = true
			}
		}
	}

	if !launchAsUser {
		// SYSTEM path
		home = resolveWindowsHomeDir()
		userEnv = os.Environ()
	}

	if home != "" {
		if st, err := os.Stat(home); err != nil || !st.IsDir() {
			home = ""
		}
	}

	if runAsUser && !launchAsUser {
		SendTerminalInfo(agentID, sessionID,
			"Run as user was not available. Falling back to SYSTEM.", nc)
	}

	opts := winpty.Options{
		DLLPrefix:   prefix,
		Command:     cmdline,
		Dir:         home,
		Env:         userEnv,
		InitialCols: 120,
		InitialRows: 30,
	}

	// impersonate -> OpenWithOptions -> revert

	// winpty.OpenWithOptions calls CreateProcess internally so we cannot pass a
	// token directly (unlike the ConPTY path which uses CreateProcessAsUser).
	// Instead we impersonate the interactive user token on the current OS thread
	// immediately before the call, then revert straight after.

	impersonationActive := false

	if launchAsUser {
		runtime.LockOSThread()

		if iErr := impersonateLoggedOnUser(windows.Token(userToken.Token())); iErr != nil {
			// Impersonation never took effect, unlock immediately, no revert needed.
			runtime.UnlockOSThread()
			userToken.Close()
			userToken = nil
			launchAsUser = false

			fmt.Printf("[WARN] ImpersonateLoggedOnUser failed for session=%s: %v. Falling back to SYSTEM.\n", sessionID, iErr)
			SendTerminalInfo(agentID, sessionID,
				"Impersonation failed. Falling back to SYSTEM.", nc)

			opts.Env = os.Environ()
			opts.Dir = resolveWindowsHomeDir()
		} else {
			impersonationActive = true
		}
	}

	// Panic-safe backstop fires ONLY if OpenWithOptions panics.
	// On the normal path, the explicit block below runs first and sets
	// impersonationActive = false, so this defer becomes a no-op.
	defer func() {
		if impersonationActive {
			revertToSelf()
			runtime.UnlockOSThread()
		}
	}()

	wp, openErr := winpty.OpenWithOptions(opts)

	// revert + unlock immediately so impersonation does not bleed into session registration or goroutine launches below.
	// revertToSelf -> UnlockOSThread -> Close token (order is mandatory).
	if impersonationActive {
		revertToSelf()
		runtime.UnlockOSThread()
		impersonationActive = false // disarms the defer above
	}

	if launchAsUser && userToken != nil {
		userToken.Close()
		userToken = nil
	}

	if openErr != nil {
		return fmt.Errorf(
			"winpty open failed (cmd=%q dir=%q prefix=%q): %+v",
			opts.Command, opts.Dir, opts.DLLPrefix, openErr,
		)
	}

	// If anything fails after we opened WinPTY, close it to avoid leaking winpty-agent.exe
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

	winTermMu.Lock()
	winTerms[sessionID] = sess
	winTermMu.Unlock()

	applyPendingResizeWindows(sessionID)

	cleanup = false

	go streamTerminalOutputWindows(agentID, sessionID, sess.wpOut, nc)

	procHandle := sess.proc

	go func() {
		if procHandle != 0 {
			_, _ = windows.WaitForSingleObject(procHandle, windows.INFINITE)

			var code uint32
			_ = windows.GetExitCodeProcess(procHandle, &code)

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
	exe, err := resolveWindowsShellExe(shell)
	if err != nil {
		exe = getCMDExe()
	}
	return quoteIfNeeded(exe)
}

// winptyPrefixDir returns the directory where winpty.dll and winpty-agent.exe are located. ( local go run main.go workaround )
// func winptyPrefixDir() string {
// 	if v := strings.TrimSpace(`C:\Users\Administrator\rmmagent`); v != "" {
// 		return v
// 	}
// 	return "."
// }
