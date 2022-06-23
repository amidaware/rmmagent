package tactical_test

import (
	"testing"

	"github.com/amidaware/rmmagent/agent/tactical"
)

func TestGetVersion(t *testing.T) {
	version := tactical.GetVersion()
	t.Logf("got version %s", version)
}
