/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	rmm "github.com/amidaware/rmmagent/shared"
	"github.com/gonutz/w32/v2"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func (a *Agent) GetEventLog(logName string, searchLastDays int) []rmm.EventLogMsg {
	var (
		oldestLog uint32
		nextSize  uint32
		readBytes uint32
	)
	buf := []byte{0}
	size := uint32(1)

	ret := make([]rmm.EventLogMsg, 0)
	startTime := time.Now().Add(time.Duration(-(time.Duration(searchLastDays)) * (24 * time.Hour)))

	h := w32.OpenEventLog("", logName)
	defer w32.CloseEventLog(h)

	numRecords, _ := w32.GetNumberOfEventLogRecords(h)
	GetOldestEventLogRecord(h, &oldestLog)

	startNum := numRecords + oldestLog - 1
	uid := 0
	for i := startNum; i >= oldestLog; i-- {
		flags := EVENTLOG_BACKWARDS_READ | EVENTLOG_SEEK_READ

		err := ReadEventLog(h, flags, i, &buf[0], size, &readBytes, &nextSize)
		if err != nil {
			if err != windows.ERROR_INSUFFICIENT_BUFFER {
				a.Logger.Debugln(err)
				break
			}
			buf = make([]byte, nextSize)
			size = nextSize
			err = ReadEventLog(h, flags, i, &buf[0], size, &readBytes, &nextSize)
			if err != nil {
				a.Logger.Debugln(err)
				break
			}

		}

		r := *(*EVENTLOGRECORD)(unsafe.Pointer(&buf[0]))

		timeWritten := time.Unix(int64(r.TimeWritten), 0)
		if searchLastDays != 0 {
			if timeWritten.Before(startTime) {
				break
			}
		}

		eventID := r.EventID & 0x0000FFFF
		sourceName, _ := bytesToString(buf[unsafe.Sizeof(EVENTLOGRECORD{}):])
		eventType := getEventType(r.EventType)

		off := uint32(0)
		args := make([]*byte, uintptr(r.NumStrings)*unsafe.Sizeof((*uint16)(nil)))
		for n := 0; n < int(r.NumStrings); n++ {
			args[n] = &buf[r.StringOffset+off]
			_, boff := bytesToString(buf[r.StringOffset+off:])
			off += boff + 2
		}

		var argsptr uintptr
		if r.NumStrings > 0 {
			argsptr = uintptr(unsafe.Pointer(&args[0]))
		}
		message, _ := getResourceMessage(logName, sourceName, r.EventID, argsptr)

		uid++
		eventLogMsg := rmm.EventLogMsg{
			Source:    sourceName,
			EventType: eventType,
			EventID:   eventID,
			Message:   message,
			Time:      timeWritten.String(),
			UID:       uid,
		}
		ret = append(ret, eventLogMsg)
	}
	return ret
}

func getEventType(et uint16) string {
	switch et {
	case windows.EVENTLOG_INFORMATION_TYPE:
		return "INFO"
	case windows.EVENTLOG_WARNING_TYPE:
		return "WARNING"
	case windows.EVENTLOG_ERROR_TYPE:
		return "ERROR"
	case windows.EVENTLOG_SUCCESS:
		return "SUCCESS"
	case windows.EVENTLOG_AUDIT_SUCCESS:
		return "AUDIT_SUCCESS"
	case windows.EVENTLOG_AUDIT_FAILURE:
		return "AUDIT_FAILURE"
	default:
		return "Unknown"
	}
}

// https://github.com/mackerelio/go-check-plugins/blob/ad7910fdc45ccb892b5e5fda65ba0956c2b2885d/check-windows-eventlog/lib/check-windows-eventlog.go#L219
func bytesToString(b []byte) (string, uint32) {
	var i int
	s := make([]uint16, len(b)/2)
	for i = range s {
		s[i] = uint16(b[i*2]) + uint16(b[(i*2)+1])<<8
		if s[i] == 0 {
			s = s[0:i]
			break
		}
	}
	return string(utf16.Decode(s)), uint32(i * 2)
}

// https://github.com/mackerelio/go-check-plugins/blob/ad7910fdc45ccb892b5e5fda65ba0956c2b2885d/check-windows-eventlog/lib/check-windows-eventlog.go#L232
func getResourceMessage(providerName, sourceName string, eventID uint32, argsptr uintptr) (string, error) {
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

	handlePtr, err := windows.UTF16PtrFromString(val)
	if err != nil {
		return "", err
	}
	handle, err := LoadLibraryEx(handlePtr, 0, DONT_RESOLVE_DLL_REFERENCES|LOAD_LIBRARY_AS_DATAFILE)
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
	message, _ := bytesToString(msgbuf[:numChars*2])
	message = strings.ReplaceAll(message, "\r", "")
	message = strings.TrimSuffix(message, "\n")
	return message, nil
}
