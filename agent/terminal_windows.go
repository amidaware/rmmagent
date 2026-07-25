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
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/fourcorelabs/wintoken"
	winpty "github.com/iamacarpet/go-winpty"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
	"github.com/ugorji/go/codec"
	"golang.org/x/sys/windows"
)

type winTerminalSession struct {
	id      string
	backend string // "conpty" or "winpty"
	job     windows.Handle
	// conpty
	hPC  windows.Handle
	proc windows.Handle
	inW  *os.File
	outR *os.File
	// winpty
	wp        *winpty.WinPTY
	wpIn      *os.File
	wpOut     *os.File
	closeOnce sync.Once
}

var (
	winTermMu         sync.Mutex
	winTerms          = map[string]*winTerminalSession{}
	pendingWinResizes = map[string]pendingWinResize{}
)

type pendingWinResize struct {
	rows int
	cols int
}

func storePendingResizeWindows(sessionID string, rows, cols int) {
	if rows <= 0 || cols <= 0 {
		return
	}

	winTermMu.Lock()
	defer winTermMu.Unlock()
	pendingWinResizes[sessionID] = pendingWinResize{
		rows: rows,
		cols: cols,
	}
}

func popPendingResizeWindows(sessionID string) (int, int, bool) {
	winTermMu.Lock()
	defer winTermMu.Unlock()

	r, ok := pendingWinResizes[sessionID]
	if !ok {
		return 0, 0, false
	}

	delete(pendingWinResizes, sessionID)
	return r.rows, r.cols, true
}

func applyPendingResizeWindows(sessionID string, logger *logrus.Logger) {
	rows, cols, ok := popPendingResizeWindows(sessionID)
	if !ok {
		return
	}
	if err := ResizeTerminalSessionWindows(sessionID, rows, cols); err != nil {
		logger.Debugf("applyPendingResizeWindows failed: session=%s rows=%d cols=%d err=%v", sessionID, rows, cols, err)
	}
}

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

func CreateProcessAsUser(
	token syscall.Token,
	applicationName *uint16,
	commandLine *uint16,
	processAttributes *windows.SecurityAttributes,
	threadAttributes *windows.SecurityAttributes,
	inheritHandles bool,
	creationFlags uint32,
	environment *uint16,
	currentDirectory *uint16,
	startupInfo *windows.StartupInfo,
	processInformation *windows.ProcessInformation,
) error {
	var inherit uintptr
	if inheritHandles {
		inherit = 1
	}

	ret, _, err := procCreateProcessAsUserW.Call(
		uintptr(token),
		uintptr(unsafe.Pointer(applicationName)),
		uintptr(unsafe.Pointer(commandLine)),
		uintptr(unsafe.Pointer(processAttributes)),
		uintptr(unsafe.Pointer(threadAttributes)),
		inherit,
		uintptr(creationFlags),
		uintptr(unsafe.Pointer(environment)),
		uintptr(unsafe.Pointer(currentDirectory)),
		uintptr(unsafe.Pointer(startupInfo)),
		uintptr(unsafe.Pointer(processInformation)),
	)
	if ret == 0 {
		if err != nil && err != syscall.Errno(0) {
			return err
		}
		return syscall.EINVAL
	}
	return nil
}

func getUserToken() (*wintoken.Token, error) {
	token, err := wintoken.GetInteractiveToken(wintoken.TokenLinked)
	if err == nil {
		return token, nil
	}
	return wintoken.GetInteractiveToken(wintoken.TokenPrimary)
}

func startTerminalSessionConPTY(agentID string, sessionID string, shell string, runAsUser bool, nc *nats.Conn, logger *logrus.Logger) error {
	if sessionID == "" {
		return fmt.Errorf("missing session_id")
	}

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

	hPC, err := createPseudoConsole(120, 30, inR, outW)
	if err != nil {
		cleanupHandles()
		return fmt.Errorf("create pseudoconsole: %w", err)
	}

	_ = windows.CloseHandle(inR)
	_ = windows.CloseHandle(outW)
	inWFile := os.NewFile(uintptr(inW), "conpty-in")
	outRFile := os.NewFile(uintptr(outR), "conpty-out")

	sess := &winTerminalSession{
		id:      sessionID,
		backend: "conpty",
		hPC:     hPC,
		proc:    0,
		inW:     inWFile,
		outR:    outRFile,
	}

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
	applyPendingResizeWindows(sessionID, logger)

	cleanupRegistered := func() {
		_ = StopTerminalSessionWindows(sessionID)
	}

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

	var (
		token        *wintoken.Token
		envBlock     *uint16
		launchAsUser = false
	)

	if runAsUser {
		token, err = getUserToken()
		if err != nil {
			logger.Debugf("terminal user token unavailable for session=%s: %v. Falling back to SYSTEM.", sessionID, err)
		} else {
			launchAsUser = true
			defer token.Close()

			envBlock, err = CreateEnvironmentBlock(syscall.Token(token.Token()))
			if err != nil {
				logger.Debugf("terminal user environment unavailable for session=%s: %v. Falling back to SYSTEM.", sessionID, err)
				launchAsUser = false
			} else {
				defer DestroyEnvironmentBlock(envBlock)

				// get users homedir
				var size uint32
				_ = windows.GetUserProfileDirectory(windows.Token(token.Token()), nil, &size)
				if size > 0 {
					buf := make([]uint16, size)
					if err := windows.GetUserProfileDirectory(windows.Token(token.Token()), &buf[0], &size); err == nil {
						userHome := windows.UTF16ToString(buf)
						cwd = windows.StringToUTF16Ptr(userHome)
					}
				}
			}
		}
	}

	if runAsUser && !launchAsUser {
		SendTerminalInfo(
			agentID,
			sessionID,
			"Run as user was not available. Falling back to SYSTEM.",
			nc,
		)
	}

	cmdline := windows.StringToUTF16Ptr(quoteIfNeeded(exe))
	var pi windows.ProcessInformation

	if launchAsUser {
		err = CreateProcessAsUser(
			syscall.Token(token.Token()),
			nil,
			cmdline,
			nil,
			nil,
			true,
			EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_UNICODE_ENVIRONMENT,
			envBlock,
			cwd,
			&siEx.StartupInfo,
			&pi,
		)
	} else {
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
	}
	if err != nil {
		cleanupRegistered()
		return fmt.Errorf("CreateProcess: %w", err)
	}

	_ = windows.CloseHandle(pi.Thread)
	sess.proc = pi.Process
	job, jobErr := createTerminalJobObject()
	if jobErr != nil {
		logger.Errorf("terminal job object creation failed: session=%s err=%v", sessionID, jobErr)
		_ = windows.TerminateProcess(pi.Process, 1)
		cleanupRegistered()
		return fmt.Errorf("failed to create terminal cleanup guard: %w", jobErr)
	}

	if assignErr := windows.AssignProcessToJobObject(job, pi.Process); assignErr != nil {
		logger.Errorf("terminal job assignment failed: session=%s err=%v", sessionID, assignErr)
		_ = windows.TerminateProcess(pi.Process, 1)
		_ = windows.CloseHandle(job)
		cleanupRegistered()
		return fmt.Errorf("failed to assign terminal cleanup guard: %w", assignErr)
	}
	sess.job = job

	go streamTerminalOutputWindows(agentID, sessionID, outRFile, nc, logger)
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

// StopTerminalSessionWindows func returns true when session is removed.
func StopTerminalSessionWindows(sessionID string) bool {
	winTermMu.Lock()
	sess, ok := winTerms[sessionID]
	if ok {
		delete(winTerms, sessionID)
		delete(pendingWinResizes, sessionID)
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
		delete(pendingWinResizes, sessionID)
	}
	winTermMu.Unlock()

	if !ok {
		return nil
	}

	if sess.job != 0 {
		_ = windows.TerminateJobObject(sess.job, 1)
	} else if sess.proc != 0 {
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
		storePendingResizeWindows(sessionID, rows, cols)
		return nil
	}

	if sess.backend == "winpty" {
		if sess.wp == nil {
			storePendingResizeWindows(sessionID, rows, cols)
			return nil
		}
		sess.wp.SetSize(uint32(cols), uint32(rows))
		return nil
	}

	if sess.hPC == 0 {
		storePendingResizeWindows(sessionID, rows, cols)
		return nil
	}
	return resizePseudoConsole(sess.hPC, int16(cols), int16(rows))
}

func streamTerminalOutputWindows(agentID, sessionID string, out *os.File, nc *nats.Conn, logger *logrus.Logger) {
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
				logger.Debugf("terminal stream read error: session=%s err=%v", sessionID, err)
			}
			return
		}

		var resp []byte
		enc := codec.NewEncoderBytes(&resp, &mh)
		if err := enc.Encode(buf[:n]); err != nil {
			logger.Errorf("terminal msgpack encode failed: session=%s err=%v", sessionID, err)
			return
		}
		if err := nc.Publish(topic, resp); err != nil {
			logger.Errorf("terminal NATS publish failed: session=%s err=%v", sessionID, err)
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

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type jobObjectExtendedLimitInfo struct {
	BasicLimitInformation jobObjectBasicLimitInfo
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

const (
	jobObjectExtendedLimitInformation = 9
	jobObjectLimitKillOnJobClose      = 0x00002000
)

// createTerminalJobObject func is used for cleaning up all child processes when the terminal session ends.
func createTerminalJobObject() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, fmt.Errorf("CreateJobObject: %w", err)
	}

	info := jobObjectExtendedLimitInfo{}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose

	if _, err := windows.SetInformationJobObject(
		job,
		jobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return 0, fmt.Errorf("SetInformationJobObject: %w", err)
	}

	return job, nil
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
		if sess.job != 0 {
			_ = windows.TerminateJobObject(sess.job, 1)
			_ = windows.CloseHandle(sess.job)
			sess.job = 0
		}
		// conpty
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
		// winpty
		if sess.wp != nil {
			sess.wp.Close()
			sess.wp = nil
		}
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
