/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/gonutz/w32/v2"
	"golang.org/x/sys/windows"
)

var _ unsafe.Pointer

var (
	modadvapi32 = windows.NewLazySystemDLL("advapi32.dll")
	modkernel32 = windows.NewLazySystemDLL("kernel32.dll")
	userenv     = windows.NewLazyDLL("userenv.dll")

	procFormatMessageW          = modkernel32.NewProc("FormatMessageW")
	procGetOldestEventLogRecord = modadvapi32.NewProc("GetOldestEventLogRecord")
	procLoadLibraryExW          = modkernel32.NewProc("LoadLibraryExW")
	procReadEventLogW           = modadvapi32.NewProc("ReadEventLogW")
	procCreateEnvironmentBlock  = userenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvironmentBlock = userenv.NewProc("DestroyEnvironmentBlock")
	procCreateProcessAsUserW    = modadvapi32.NewProc("CreateProcessAsUserW")
)

// https://docs.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-eventlogrecord
type EVENTLOGRECORD struct {
	Length              uint32
	Reserved            uint32
	RecordNumber        uint32
	TimeGenerated       uint32
	TimeWritten         uint32
	EventID             uint32
	EventType           uint16
	NumStrings          uint16
	EventCategory       uint16
	ReservedFlags       uint16
	ClosingRecordNumber uint32
	StringOffset        uint32
	UserSidLength       uint32
	UserSidOffset       uint32
	DataLength          uint32
	DataOffset          uint32
}

type ReadFlag uint32

const (
	EVENTLOG_SEQUENTIAL_READ ReadFlag = 1 << iota
	EVENTLOG_SEEK_READ
	EVENTLOG_FORWARDS_READ
	EVENTLOG_BACKWARDS_READ
)

const (
	DONT_RESOLVE_DLL_REFERENCES uint32 = 0x0001
	LOAD_LIBRARY_AS_DATAFILE    uint32 = 0x0002
)

func FormatMessage(flags uint32, source syscall.Handle, messageID uint32, languageID uint32, buffer *byte, bufferSize uint32, arguments uintptr) (numChars uint32, err error) {
	r0, _, e1 := syscall.Syscall9(procFormatMessageW.Addr(), 7, uintptr(flags), uintptr(source), uintptr(messageID), uintptr(languageID), uintptr(unsafe.Pointer(buffer)), uintptr(bufferSize), uintptr(arguments), 0, 0)
	numChars = uint32(r0)
	if numChars == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func GetOldestEventLogRecord(eventLog w32.HANDLE, oldestRecord *uint32) (err error) {
	r1, _, e1 := syscall.Syscall(procGetOldestEventLogRecord.Addr(), 2, uintptr(eventLog), uintptr(unsafe.Pointer(oldestRecord)), 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func LoadLibraryEx(filename *uint16, file syscall.Handle, flags uint32) (handle syscall.Handle, err error) {
	r0, _, e1 := syscall.Syscall(procLoadLibraryExW.Addr(), 3, uintptr(unsafe.Pointer(filename)), uintptr(file), uintptr(flags))
	handle = syscall.Handle(r0)
	if handle == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func ReadEventLog(eventLog w32.HANDLE, readFlags ReadFlag, recordOffset uint32, buffer *byte, numberOfBytesToRead uint32, bytesRead *uint32, minNumberOfBytesNeeded *uint32) (err error) {
	r1, _, e1 := syscall.Syscall9(procReadEventLogW.Addr(), 7, uintptr(eventLog), uintptr(readFlags), uintptr(recordOffset), uintptr(unsafe.Pointer(buffer)), uintptr(numberOfBytesToRead), uintptr(unsafe.Pointer(bytesRead)), uintptr(unsafe.Pointer(minNumberOfBytesNeeded)), 0, 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func CreateEnvironmentBlock(token syscall.Token) (*uint16, error) {
	var envBlock *uint16

	ret, _, err := procCreateEnvironmentBlock.Call(
		uintptr(unsafe.Pointer(&envBlock)),
		uintptr(token),
		0,
	)
	if ret == 0 {
		return nil, err
	}

	return envBlock, nil
}

func DestroyEnvironmentBlock(envBlock *uint16) error {
	ret, _, err := procDestroyEnvironmentBlock.Call(uintptr(unsafe.Pointer(envBlock)))
	if ret == 0 {
		return err
	}
	return nil
}

func EnvironmentBlockToSlice(envBlock *uint16) []string {
	var envs []string

	for {
		len := 0
		for *(*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(envBlock)) + uintptr(len*2))) != 0 {
			len++
		}

		if len == 0 {
			break
		}

		env := syscall.UTF16ToString((*[1 << 29]uint16)(unsafe.Pointer(envBlock))[:len])
		envs = append(envs, env)
		envBlock = (*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(envBlock)) + uintptr((len+1)*2)))
	}

	return envs
}

func openFileNoFollow(path string, flag int, perm os.FileMode) (*os.File, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	access := uint32(windows.GENERIC_READ | windows.GENERIC_WRITE)
	share := uint32(windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE)

	var createmode uint32
	switch {
	case flag&(os.O_CREATE|os.O_EXCL) == (os.O_CREATE | os.O_EXCL):
		createmode = windows.CREATE_NEW
	case flag&os.O_CREATE != 0 && flag&os.O_TRUNC != 0:
		createmode = windows.CREATE_ALWAYS
	case flag&os.O_CREATE != 0:
		createmode = windows.OPEN_ALWAYS
	default:
		createmode = windows.OPEN_EXISTING
	}

	attrs := uint32(windows.FILE_ATTRIBUTE_NORMAL | windows.FILE_FLAG_OPEN_REPARSE_POINT)
	h, err := windows.CreateFile(
		pathPtr,
		access,
		share,
		nil,
		createmode,
		attrs,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}

const privateFileSDDL = "D:P(A;;FA;;;SY)(A;;FA;;;BA)"

func createPrivateFileNoFollow(path string) (*os.File, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	sd, err := windows.SecurityDescriptorFromString(privateFileSDDL)
	if err != nil {
		return nil, err
	}
	sa := windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}
	h, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		uint32(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE),
		&sa,
		windows.CREATE_NEW,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}

func hasExtraHardLinks(_ *os.File) bool { return false }

func openExistingFileSharedRead(path string) (*os.File, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	h, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ,
		uint32(windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE),
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(h), path), nil
}
