package utils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/amidaware/rmmagent/shared"
	"github.com/go-resty/resty/v2"
)

func CaptureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func ByteCountSI(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

// CleanString removes invalid utf-8 byte sequences
func CleanString(s string) string {
	r := strings.NewReplacer("\x00", "")
	s = r.Replace(s)
	return strings.ToValidUTF8(s, "")
}

func RemoveWinNewLines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func CreateTmpFile() (*os.File, error) {
	var f *os.File
	f, err := os.CreateTemp("", "trmm")
	if err != nil {
		cwd, err := os.Getwd()
		if err != nil {
			return f, err
		}

		f, err = os.CreateTemp(cwd, "trmm")
		if err != nil {
			return f, err
		}
	}

	return f, nil
}

func WebRequest(requestType string, timeout time.Duration, payload map[string]string, url string, proxy string) (response resty.Response, err error) {
	client := resty.New()
	client.SetTimeout(timeout * time.Second)
	client.SetCloseConnection(true)
	if shared.DEBUG {
		client.SetDebug(true)
	}

	result, err := client.R().Get(url)
	return *result, err	
}

// StripAll strips all whitespace and newline chars
func StripAll(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\n")
	s = strings.Trim(s, "\r")
	return s
}