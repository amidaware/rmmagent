//go:build windows

package agent

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unsafe"

	winpty "github.com/iamacarpet/go-winpty"
	"github.com/nats-io/nats.go"
	"github.com/ugorji/go/codec"
	"golang.org/x/sys/windows"
)

type winTerminalSession struct {
	id string

	backend string // "conpty" | "winpty"

	// ConPTY
	hPC  windows.Handle
	proc windows.Handle
	inW  *os.File
	outR *os.File

	// WinPTY
	wp    *winpty.WinPTY
	wpIn  *os.File
	wpOut *os.File

	closeOnce sync.Once
}

var (
	winTermMu sync.Mutex
	winTerms  = map[string]*winTerminalSession{}
)

func resolveWindowsHomeDir() string {
	if v := strings.TrimSpace(os.Getenv("HOME")); v != "" {
		return v
	}

	if v := strings.TrimSpace(os.Getenv("USERPROFILE")); v != "" {
		return v
	}

	hd := strings.TrimSpace(os.Getenv("HOMEDRIVE"))
	hp := strings.TrimSpace(os.Getenv("HOMEPATH"))
	if hd != "" && hp != "" {
		return hd + hp
	}
	return ""
}

func resolveWindowsShellExe(shell string) (string, error) {
	s := strings.TrimSpace(shell)
	sl := strings.ToLower(s)

	switch sl {
	case "":
		return getCMDExe(), nil
	case "cmd", "cmd.exe":
		return getCMDExe(), nil
	case "powershell", "powershell.exe":
		return getPowershellExe(), nil
	default:
		s = filepath.Clean(s)

		if !isAbsoluteWindowsExePath(s) {
			return "", fmt.Errorf("invalid custom shell path: must be an absolute .exe path")
		}

		if _, err := os.Stat(s); err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("custom shell not found: %s", s)
			}
			return "", fmt.Errorf("custom shell is not accessible: %s", s)
		}

		return s, nil
	}
}

func isAbsoluteWindowsExePath(path string) bool {
	p := strings.TrimSpace(path)
	if p == "" {
		return false
	}

	if strings.ContainsAny(p, "\"\n\r") {
		return false
	}

	p = filepath.Clean(p)

	if !filepath.IsAbs(p) {
		return false
	}

	if !strings.EqualFold(filepath.Ext(p), ".exe") {
		return false
	}

	return true
}

func startTerminalSessionConPTY(agentID string, sessionID string, shell string, nc *nats.Conn) error {
	if sessionID == "" {
		return fmt.Errorf("missing session_id")
	}

	// Prevent duplicates
	winTermMu.Lock()
	if _, exists := winTerms[sessionID]; exists {
		winTermMu.Unlock()
		return fmt.Errorf("session already exists: %s", sessionID)
	}
	winTermMu.Unlock()

	exe, err := resolveWindowsShellExe(shell)
	if err != nil {
		return err
	}

	// Create inheritable pipes
	inR, inW, err := createInheritablePipe()
	if err != nil {
		return fmt.Errorf("create input pipe: %w", err)
	}
	outR, outW, err := createInheritablePipe()
	if err != nil {
		_ = windows.CloseHandle(inR)
		_ = windows.CloseHandle(inW)
		return fmt.Errorf("create output pipe: %w", err)
	}

	cleanupHandles := func() {
		_ = windows.CloseHandle(inR)
		_ = windows.CloseHandle(inW)
		_ = windows.CloseHandle(outR)
		_ = windows.CloseHandle(outW)
	}

	// Create ConPTY (initial size)
	hPC, err := createPseudoConsole(120, 30, inR, outW)
	if err != nil {
		cleanupHandles()
		return fmt.Errorf("create pseudoconsole: %w", err)
	}

	// ConPTY uses inR + outW; we use inW + outR
	_ = windows.CloseHandle(inR)
	_ = windows.CloseHandle(outW)

	inWFile := os.NewFile(uintptr(inW), "conpty-in")
	outRFile := os.NewFile(uintptr(outR), "conpty-out")

	// Create session object NOW (proc will be set after CreateProcess)
	sess := &winTerminalSession{
		id:      sessionID,
		backend: "conpty",
		hPC:     hPC,
		proc:    0,
		inW:     inWFile,
		outR:    outRFile,
	}

	// Register early so resize won't race (only needs hPC)
	winTermMu.Lock()
	if _, exists := winTerms[sessionID]; exists {
		winTermMu.Unlock()

		_ = inWFile.Close()
		_ = outRFile.Close()
		closePseudoConsole(hPC)

		return fmt.Errorf("session already exists: %s", sessionID)
	}
	winTerms[sessionID] = sess
	winTermMu.Unlock()

	// From here onward: on any failure, remove session + cleanup via Stop()
	cleanupRegistered := func() {
		_ = StopTerminalSessionWindows(sessionID)
	}

	// Build STARTUPINFOEX
	siEx, attr, err := buildStartupInfoEx(hPC)
	if err != nil {
		cleanupRegistered()
		return fmt.Errorf("build startupinfoex: %w", err)
	}
	defer deleteProcThreadAttrList(attr)

	home := resolveWindowsHomeDir()
	if home != "" {
		if st, err := os.Stat(home); err != nil || !st.IsDir() {
			home = ""
		}
	}

	var cwd *uint16
	if home != "" {
		cwd = windows.StringToUTF16Ptr(home)
	}

	// Create process attached to pseudo console
	cmdline := windows.StringToUTF16Ptr(quoteIfNeeded(exe))
	var pi windows.ProcessInformation

	err = windows.CreateProcess(
		nil,
		cmdline,
		nil,
		nil,
		true,
		EXTENDED_STARTUPINFO_PRESENT,
		nil,
		cwd,
		&siEx.StartupInfo,
		&pi,
	)
	if err != nil {
		cleanupRegistered()
		return fmt.Errorf("CreateProcess: %w", err)
	}

	_ = windows.CloseHandle(pi.Thread)

	// Save proc handle into session (now kill/watcher can use it)
	sess.proc = pi.Process

	// Stream output
	go streamTerminalOutputWindows(agentID, sessionID, outRFile, nc)

	// Exit watcher
	go func() {
		_, _ = windows.WaitForSingleObject(sess.proc, windows.INFINITE)

		var code uint32
		_ = windows.GetExitCodeProcess(sess.proc, &code)

		removed := StopTerminalSessionWindows(sessionID)
		if removed {
			sendTerminalDoneWindows(agentID, sessionID, int(code), nc)
		}
	}()

	return nil
}

// StopTerminalSessionWindows returns true if it actually removed a session (useful for watcher/kill coordination)
func StopTerminalSessionWindows(sessionID string) bool {
	winTermMu.Lock()
	sess, ok := winTerms[sessionID]
	if ok {
		delete(winTerms, sessionID)
	}
	winTermMu.Unlock()

	if !ok {
		return false
	}
	cleanupWinSession(sess)
	return true
}

func KillTerminalSessionWindows(sessionID string) error {
	winTermMu.Lock()
	sess, ok := winTerms[sessionID]
	if ok {
		delete(winTerms, sessionID)
	}
	winTermMu.Unlock()

	if !ok {
		return nil
	}

	// Hard kill if we have a process handle (works for both backends)
	if sess.proc != 0 {
		_ = windows.TerminateProcess(sess.proc, 1)
	}
	cleanupWinSession(sess)
	return nil
}

func FeedTerminalInputWindows(sessionID string, input string) error {
	winTermMu.Lock()
	sess, ok := winTerms[sessionID]
	winTermMu.Unlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if sess.backend == "winpty" {
		if sess.wpIn == nil {
			return fmt.Errorf("stdin not initialized for winpty session: %s", sessionID)
		}
		_, err := sess.wpIn.Write([]byte(input))
		return err
	}

	// ConPTY path (your existing)
	if sess.inW == nil {
		return fmt.Errorf("stdin pipe not initialized for session: %s", sessionID)
	}
	_, err := sess.inW.Write([]byte(input))
	return err
}

func ResizeTerminalSessionWindows(sessionID string, rows, cols int) error {
	if rows <= 0 || cols <= 0 {
		return nil
	}

	winTermMu.Lock()
	sess, ok := winTerms[sessionID]
	winTermMu.Unlock()

	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	if sess.backend == "winpty" {
		if sess.wp == nil {
			return fmt.Errorf("winpty not initialized for session: %s", sessionID)
		}
		// winpty expects (cols, rows) :contentReference[oaicite:8]{index=8}
		sess.wp.SetSize(uint32(cols), uint32(rows))
		return nil
	}

	// ConPTY path (your existing)
	if sess.hPC == 0 {
		return fmt.Errorf("pseudoconsole handle is nil for session: %s", sessionID)
	}
	return resizePseudoConsole(sess.hPC, int16(cols), int16(rows))
}

func streamTerminalOutputWindows(agentID, sessionID string, out *os.File, nc *nats.Conn) {
	topic := agentID + ".terminal." + sessionID

	var mh codec.MsgpackHandle
	buf := make([]byte, 2048)

	for {
		n, err := out.Read(buf)
		if err != nil {
			if err != io.EOF && strings.Contains(err.Error(), "file already closed") {
				// normal during cleanup/kill
				return
			}
			if err != io.EOF {
				fmt.Printf("[WARN] Stream read error: session=%s err=%v", sessionID, err)
			}
			return
		}

		var resp []byte
		enc := codec.NewEncoderBytes(&resp, &mh)
		if err := enc.Encode(buf[:n]); err != nil {
			fmt.Printf("[WARN] MSGPACK encode failed: session=%s err=%v", sessionID, err)
			return
		}
		if err := nc.Publish(topic, resp); err != nil {
			fmt.Printf("[WARN] NATS publish failed: session=%s err=%v", sessionID, err)
			return
		}
	}
}

func sendTerminalDoneWindows(agentID, sessionID string, exitCode int, nc *nats.Conn) {
	topic := agentID + ".terminal." + sessionID

	payload := map[string]interface{}{
		"done":      true,
		"exit_code": exitCode,
	}

	var resp []byte
	enc := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
	_ = enc.Encode(payload)
	_ = nc.Publish(topic, resp)
}

var (
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procCreatePseudoConsole = kernel32.NewProc("CreatePseudoConsole")
	procResizePseudoConsole = kernel32.NewProc("ResizePseudoConsole")
	procClosePseudoConsole  = kernel32.NewProc("ClosePseudoConsole")

	procInitializeProcThreadAttr  = kernel32.NewProc("InitializeProcThreadAttributeList")
	procUpdateProcThreadAttribute = kernel32.NewProc("UpdateProcThreadAttribute")
	procDeleteProcThreadAttribute = kernel32.NewProc("DeleteProcThreadAttributeList")
)

const (
	PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE = 0x00020016
	EXTENDED_STARTUPINFO_PRESENT        = 0x00080000
)

type coord struct {
	X int16
	Y int16
}

type startupInfoEx struct {
	windows.StartupInfo
	lpAttributeList *byte
}

func cleanupWinSession(sess *winTerminalSession) {
	if sess == nil {
		return
	}

	sess.closeOnce.Do(func() {
		// ConPTY
		if sess.inW != nil {
			_ = sess.inW.Close()
			sess.inW = nil
		}
		if sess.outR != nil {
			_ = sess.outR.Close()
			sess.outR = nil
		}
		if sess.hPC != 0 {
			closePseudoConsole(sess.hPC)
			sess.hPC = 0
		}
		if sess.proc != 0 {
			_ = windows.CloseHandle(sess.proc)
			sess.proc = 0
		}

		// WinPTY
		if sess.wp != nil {
			sess.wp.Close()
			sess.wp = nil
		}
		// These may already be closed by wp.Close(), but safe to suppress errors.
		if sess.wpIn != nil {
			_ = sess.wpIn.Close()
			sess.wpIn = nil
		}
		if sess.wpOut != nil {
			_ = sess.wpOut.Close()
			sess.wpOut = nil
		}
	})
}


func quoteIfNeeded(s string) string {
	if strings.ContainsAny(s, " \t") && !(strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`)) {
		return `"` + s + `"`
	}
	return s
}

func createInheritablePipe() (r windows.Handle, w windows.Handle, err error) {
	sa := &windows.SecurityAttributes{
		Length:        uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle: 1,
	}
	if err = windows.CreatePipe(&r, &w, sa, 0); err != nil {
		return 0, 0, err
	}
	return r, w, nil
}

func createPseudoConsole(cols, rows int16, inR, outW windows.Handle) (windows.Handle, error) {
	var hPC windows.Handle
	c := coord{X: cols, Y: rows}
	coordPacked := *(*uint32)(unsafe.Pointer(&c))

	r1, _, e1 := procCreatePseudoConsole.Call(
		uintptr(coordPacked),
		uintptr(inR),
		uintptr(outW),
		0,
		uintptr(unsafe.Pointer(&hPC)),
	)
	if r1 != 0 {
		return 0, error(e1)
	}
	return hPC, nil
}

func resizePseudoConsole(hPC windows.Handle, cols, rows int16) error {
	c := coord{X: cols, Y: rows}
	coordPacked := *(*uint32)(unsafe.Pointer(&c))

	r1, _, e1 := procResizePseudoConsole.Call(
		uintptr(hPC),
		uintptr(coordPacked),
	)
	if r1 != 0 {
		return error(e1)
	}
	return nil
}

func closePseudoConsole(hPC windows.Handle) {
	_, _, _ = procClosePseudoConsole.Call(uintptr(hPC))
}

func buildStartupInfoEx(hPC windows.Handle) (*startupInfoEx, *byte, error) {
	var size uintptr
	_, _, _ = procInitializeProcThreadAttr.Call(0, 1, 0, uintptr(unsafe.Pointer(&size)))

	buf := make([]byte, size)
	attr := &buf[0]

	r1, _, e1 := procInitializeProcThreadAttr.Call(
		uintptr(unsafe.Pointer(attr)),
		1,
		0,
		uintptr(unsafe.Pointer(&size)),
	)
	if r1 == 0 {
		return nil, nil, error(e1)
	}

	r1, _, e1 = procUpdateProcThreadAttribute.Call(
		uintptr(unsafe.Pointer(attr)),
		0,
		uintptr(PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE),
		uintptr(hPC),
		unsafe.Sizeof(hPC),
		0,
		0,
	)
	if r1 == 0 {
		deleteProcThreadAttrList(attr)
		return nil, nil, error(e1)
	}

	var si startupInfoEx
	si.Cb = uint32(unsafe.Sizeof(si))
	si.lpAttributeList = attr
	return &si, attr, nil
}

func deleteProcThreadAttrList(attr *byte) {
	if attr == nil {
		return
	}
	_, _, _ = procDeleteProcThreadAttribute.Call(uintptr(unsafe.Pointer(attr)))
}
