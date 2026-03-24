//go:build windows

package agent

import (
	"fmt"

	nats "github.com/nats-io/nats.go"
	"github.com/ugorji/go/codec"
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

func SendTerminalError(agentID, sessionID, message string, nc *nats.Conn) {
	topic := agentID + ".terminal." + sessionID

	payload := map[string]interface{}{
		"output":     "[ERROR] " + message + "\r\n",
		"session_id": sessionID,
		"done":       true,
		"exit_code":  1,
	}

	var resp []byte
	enc := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
	_ = enc.Encode(payload)
	_ = nc.Publish(topic, resp)
}
