//go:build windows

package agent

import (
	"fmt"

	nats "github.com/nats-io/nats.go"
)

func conptySupported() bool {
	// procCreatePseudoConsole exists in your ConPTY file (terminal_windows.go)
	return procCreatePseudoConsole.Find() == nil
}

func StartTerminalSessionWindows(agentID, sessionID, shell string, nc *nats.Conn) error {
	if sessionID == "" {
		return fmt.Errorf("missing session_id")
	}

	if conptySupported() {
		return startTerminalSessionConPTY(agentID, sessionID, shell, nc)
	}
	return startTerminalSessionWinPTY(agentID, sessionID, shell, nc)
}
