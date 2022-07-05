package syscall

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/gonutz/w32/v2"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

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

var (
	modadvapi32 = windows.NewLazySystemDLL("advapi32.dll")
	modkernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procFormatMessageW          = modkernel32.NewProc("FormatMessageW")
	procGetOldestEventLogRecord = modadvapi32.NewProc("GetOldestEventLogRecord")
	procLoadLibraryExW          = modkernel32.NewProc("LoadLibraryExW")
	procReadEventLogW           = modadvapi32.NewProc("ReadEventLogW")
)

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

// https://github.com/mackerelio/go-check-plugins/blob/ad7910fdc45ccb892b5e5fda65ba0956c2b2885d/check-windows-eventlog/lib/check-windows-eventlog.go#L232
func GetResourceMessage(providerName, sourceName string, eventID uint32, argsptr uintptr) (string, error) {
	regkey := fmt.Sprintf(
		"SYSTEM\\CurrentControlSet\\Services\\EventLog\\%s\\%s",
		providerName, sourceName)
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, regkey, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}

	defer key.Close()
	val, _, err := key.GetStringValue("EventMessageFile")
	if err != nil {
		return "", err
	}

	val, err = registry.ExpandString(val)
	if err != nil {
		return "", err
	}

	handle, err := LoadLibraryEx(syscall.StringToUTF16Ptr(val), 0,
		DONT_RESOLVE_DLL_REFERENCES|LOAD_LIBRARY_AS_DATAFILE)
	if err != nil {
		return "", err
	}

	defer syscall.CloseHandle(handle)

	msgbuf := make([]byte, 1<<16)
	numChars, err := FormatMessage(
		syscall.FORMAT_MESSAGE_FROM_SYSTEM|
			syscall.FORMAT_MESSAGE_FROM_HMODULE|
			syscall.FORMAT_MESSAGE_ARGUMENT_ARRAY,
		handle,
		eventID,
		0,
		&msgbuf[0],
		uint32(len(msgbuf)),
		argsptr)
	if err != nil {
		return "", err
	}

	message, _ := utils.BytesToString(msgbuf[:numChars*2])
	message = strings.Replace(message, "\r", "", -1)
	message = strings.TrimSuffix(message, "\n")
	return message, nil
}

func StringToUTF16Ptr(val string) *uint16 {
	return syscall.StringToUTF16Ptr(val)
}

func CloseHandle(handle syscall.Handle) {
	defer syscall.CloseHandle(handle)
}
