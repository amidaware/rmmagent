package utils

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/amidaware/rmmagent/shared"
	"github.com/go-resty/resty/v2"
	"github.com/shirou/gopsutil/v3/process"
	trmm "github.com/wh1te909/trmm-shared"
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

// KillProc kills a process and its children
func KillProc(pid int32) error {
	p, err := process.NewProcess(pid)
	if err != nil {
		return err
	}

	children, err := p.Children()
	if err == nil {
		for _, child := range children {
			if err := child.Kill(); err != nil {
				continue
			}
		}
	}

	if err := p.Kill(); err != nil {
		return err
	}
	return nil
}

func CreateTRMMTempDir() {
	// create the temp dir for running scripts
	dir := filepath.Join(os.TempDir(), "trmm")
	if !trmm.FileExists(dir) {
		err := os.Mkdir(dir, 0775)
		if err != nil {
			//a.Logger.Errorln(err)
		}
	}
}

func RandRange(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min) + min
}

// https://golangcode.com/unzip-files-in-go/
func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}

	defer r.Close()
	for _, f := range r.File {
		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)
		// Check for ZipSlip. More Info: http://bit.ly/2MsjAWE
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("%s: illegal file path", fpath)
		}

		if f.FileInfo().IsDir() {
			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Make File
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		_, err = io.Copy(outFile, rc)
		// Close the file without defer to close before next iteration of loop
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

func RandomCheckDelay() {
	time.Sleep(time.Duration(RandRange(300, 950)) * time.Millisecond)
}