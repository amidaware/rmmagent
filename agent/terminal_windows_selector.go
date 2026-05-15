//go:build windows

/*
Copyright 2025 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"fmt"

	nats "github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
	"github.com/ugorji/go/codec"
)

func conptySupported() bool {
	return procCreatePseudoConsole.Find() == nil
}

func StartTerminalSessionWindows(agentID, programDir, sessionID, shell string, runAsUser bool, nc *nats.Conn, logger *logrus.Logger) error {
	if sessionID == "" {
		return fmt.Errorf("missing session_id")
	}

	if conptySupported() {
		return startTerminalSessionConPTY(agentID, sessionID, shell, runAsUser, nc, logger)
	}
	return startTerminalSessionWinPTY(agentID, programDir, sessionID, shell, runAsUser, nc, logger)
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

func SendTerminalInfo(agentID, sessionID, message string, nc *nats.Conn) {
	topic := agentID + ".terminal." + sessionID

	payload := map[string]interface{}{
		"output":     "[INFO] " + message + "\r\n",
		"session_id": sessionID,
		"done":       false,
		"exit_code":  0,
	}

	var resp []byte
	enc := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))
	_ = enc.Encode(payload)
	_ = nc.Publish(topic, resp)
}
