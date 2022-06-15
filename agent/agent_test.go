package agent

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func captureOutput(f func()) string {
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

func TestShowStatus(t *testing.T) {
	var (
		version = "2.0.4"
	)

	output := captureOutput(func() {
		ShowStatus(version)
	})

	if output != (version + "\n") {
		t.Errorf("ShowStatus output not equal to version defined.")
	}
}