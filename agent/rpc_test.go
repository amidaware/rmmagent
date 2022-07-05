package agent

import (
	"testing"
)

//uncomment to test rpc, comment to add back before commit, this test will always timeout
func TestRunRPC(t *testing.T) {
	a := New(lg, version)
	t.Log(a.NatsServer)
	a.RunRPC()
}
