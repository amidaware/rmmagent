package events

import (
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/amidaware/rmmagent/agent/syscall"
	"github.com/gonutz/w32/v2"
	"golang.org/x/sys/windows"
)

func GetEventLog(logName string, searchLastDays int) ([]EventLogMsg, error) {
	var (
		oldestLog uint32
		nextSize  uint32
		readBytes uint32
	)

	buf := []byte{0}
	size := uint32(1)
	ret := make([]EventLogMsg, 0)
	startTime := time.Now().Add(time.Duration(-(time.Duration(searchLastDays)) * (24 * time.Hour)))
	h := w32.OpenEventLog("", logName)
	defer w32.CloseEventLog(h)
	numRecords, _ := w32.GetNumberOfEventLogRecords(h)
	err := syscall.GetOldestEventLogRecord(h, &oldestLog)
	startNum := numRecords + oldestLog - 1
	uid := 0
	for i := startNum; i >= oldestLog; i-- {
		flags := syscall.EVENTLOG_BACKWARDS_READ | syscall.EVENTLOG_SEEK_READ
		err := syscall.ReadEventLog(h, flags, i, &buf[0], size, &readBytes, &nextSize)
		if err != nil {
			if err != windows.ERROR_INSUFFICIENT_BUFFER {
				break
			}
			buf = make([]byte, nextSize)
			size = nextSize
			err = syscall.ReadEventLog(h, flags, i, &buf[0], size, &readBytes, &nextSize)
			if err != nil {
				break
			}
		}

		r := *(*syscall.EVENTLOGRECORD)(unsafe.Pointer(&buf[0]))

		timeWritten := time.Unix(int64(r.TimeWritten), 0)
		if searchLastDays != 0 {
			if timeWritten.Before(startTime) {
				break
			}
		}

		eventID := r.EventID & 0x0000FFFF
		sourceName, _ := bytesToString(buf[unsafe.Sizeof(syscall.EVENTLOGRECORD{}):])
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

		message, _ := syscall.GetResourceMessage(logName, sourceName, r.EventID, argsptr)

		uid++
		eventLogMsg := EventLogMsg{
			Source:    sourceName,
			EventType: eventType,
			EventID:   eventID,
			Message:   message,
			Time:      timeWritten.String(),
			UID:       uid,
		}

		ret = append(ret, eventLogMsg)
	}

	return ret, err
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
