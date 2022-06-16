package agent

import (
	"testing"
)

func TestRunRPC(t *testing.T) {
	a := New(lg, version)
	a.RunRPC()
}