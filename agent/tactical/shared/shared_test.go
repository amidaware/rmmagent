package shared_test

import (
	"testing"

	"github.com/amidaware/rmmagent/agent/tactical/shared"
)

func TestGetPythonBin(t *testing.T) {
	pybin := shared.GetPythonBin()
	if pybin == "" {
		t.Errorf("expected path, got %s", pybin)
	}

	t.Logf("result: %s", pybin)
}