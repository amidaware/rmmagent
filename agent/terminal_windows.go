//go:build windows

package agent

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"unsafe"

	"github.com/nats-io/nats.go"
	"github.com/ugorji/go/codec"
	"golang.org/x/sys/windows"
)

/*
Global session registry (because Windows uses global functions like CMDShell)
*/
type winTerminalSession struct {
	id string

	hPC windows.Handle

	// Process handle for hard kill
	proc windows.Handle

	// IO pipes exposed as *os.File
	inW  *os.File
	outR *os.File

	// used to avoid double cleanup
	closed bool
}

var (
	winTermMu sync.Mutex
	winTerms  = map[string]*winTerminalSession{}
)

// ---------------------- Public global API ----------------------

func StartTerminalSessionWindows(agentID string, sessionID string, shell string, nc *nats.Conn) error {
	fmt.Println("StartTerminalSessionWindows: agent=%s session=%s shell=%s", agentID, sessionID, shell)

	if sessionID == "" {
		return fmt.Errorf("missing session_id")
	}

	// Prevent duplicates
	winTermMu.Lock()
	if _, exists := winTerms[sessionID]; exists {
		winTermMu.Unlock()
		return fmt.Errorf("Session already exists: %s", sessionID)
	}
	winTermMu.Unlock()

	exe := pickWindowsShellExe(shell)

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

	// If anything fails after here, we must close all remaining handles/files
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

	// Build STARTUPINFOEX
	siEx, attr, err := buildStartupInfoEx(hPC)
	if err != nil {
		closePseudoConsole(hPC)
		cleanupHandles()
		return fmt.Errorf("build startupinfoex: %w", err)
	}
	defer deleteProcThreadAttrList(attr)

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
		nil,
		&siEx.StartupInfo,
		&pi,
	)
	if err != nil {
		closePseudoConsole(hPC)
		cleanupHandles()
		return fmt.Errorf("CreateProcess: %w", err)
	}

	// We don't need thread handle
	_ = windows.CloseHandle(pi.Thread)

	// ConPTY uses inR + outW; we use inW + outR
	_ = windows.CloseHandle(inR)
	_ = windows.CloseHandle(outW)

	inWFile := os.NewFile(uintptr(inW), "conpty-in")
	outRFile := os.NewFile(uintptr(outR), "conpty-out")

	sess := &winTerminalSession{
		id:   sessionID,
		hPC:  hPC,
		proc: pi.Process,
		inW:  inWFile,
		outR: outRFile,
	}

	// Store session
	winTermMu.Lock()
	// Double-check in case of race
	if _, exists := winTerms[sessionID]; exists {
		winTermMu.Unlock()
		// cleanup created process/session
		_ = windows.TerminateProcess(pi.Process, 1)
		_ = windows.CloseHandle(pi.Process)
		_ = inWFile.Close()
		_ = outRFile.Close()
		closePseudoConsole(hPC)
		return fmt.Errorf("Session already exists: %s", sessionID)
	}
	winTerms[sessionID] = sess
	winTermMu.Unlock()

	fmt.Println("Registered Windows terminal session %s", sessionID)

	// Stream output
	go streamTerminalOutputWindows(agentID, sessionID, outRFile, nc)

	// Exit watcher
	go func() {
		_, _ = windows.WaitForSingleObject(sess.proc, windows.INFINITE)

		var code uint32
		_ = windows.GetExitCodeProcess(sess.proc, &code)

		StopTerminalSessionWindows(sessionID) // safe cleanup + remove
		sendTerminalDoneWindows(agentID, sessionID, int(code), nc)
	}()

	return nil
}

func StopTerminalSessionWindows(sessionID string) {
	winTermMu.Lock()
	sess, ok := winTerms[sessionID]
	if ok {
		delete(winTerms, sessionID)
	}
	winTermMu.Unlock()
	if !ok {
		return
	}
	cleanupWinSession(sess)
}

func KillTerminalSessionWindows(sessionID string) error {
	winTermMu.Lock()
	sess, ok := winTerms[sessionID]
	if ok {
		delete(winTerms, sessionID)
	}
	winTermMu.Unlock()

	if !ok {
		// add WARN here
		fmt.Println("KillTerminalSessionWindows: session already gone: %s", sessionID)
		return nil
	}

	// hard kill process
	if sess.proc != 0 {
		_ = windows.TerminateProcess(sess.proc, 1)
	}
	// cleanup (closes proc handle too)
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
	if sess.inW == nil {
		return fmt.Errorf("stdin pipe not initialized for session: %s", sessionID)
	}

	// Xterm sends UTF-8 bytes; ConPTY stdin expects bytes as-is.
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
	if sess.hPC == 0 {
		return fmt.Errorf("pseudoconsole handle is nil for session: %s", sessionID)
	}
	return resizePseudoConsole(sess.hPC, int16(cols), int16(rows))
}

// ---------------------- Streaming + done ----------------------

func streamTerminalOutputWindows(agentID, sessionID string, out *os.File, nc *nats.Conn) {
	topic := agentID + ".terminal." + sessionID

	var mh codec.MsgpackHandle
	buf := make([]byte, 2048)

	for {
		n, err := out.Read(buf)
		if err != nil {
			if err != io.EOF {
				// add WARN here
				fmt.Println("ConPTY output closed for session %s: %v", sessionID, err)
			}
			return
		}

		var resp []byte
		enc := codec.NewEncoderBytes(&resp, &mh)
		if err := enc.Encode(buf[:n]); err != nil {
			fmt.Println("msgpack encode failed for session %s: %v", sessionID, err)
			return
		}
		if err := nc.Publish(topic, resp); err != nil {
			fmt.Println("nats publish failed for session %s: %v", sessionID, err)
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

// ---------------------- Helpers (ConPTY) ----------------------

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

	// Prevent double cleanup if called twice somehow
	if sess.closed {
		return
	}
	sess.closed = true

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
}

func pickWindowsShellExe(shell string) string {
	s := strings.ToLower(strings.TrimSpace(shell))
	switch s {
	case "", "/bin/bash", "powershell", "powershell.exe":
		return getPowershellExe()
	case "cmd", "cmd.exe":
		return getCMDExe()
	default:
		// allow full exe path
		if strings.HasSuffix(s, ".exe") {
			return shell
		}
		return getPowershellExe()
	}
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
