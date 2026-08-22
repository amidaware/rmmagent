/*
Copyright 2025 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"crypto/tls"

	"github.com/sirupsen/logrus"
)

// buildTLSConfig returns the tls config used for connections to the rmm server.
// Returns nil when there is nothing to configure, so callers can leave the
// transport defaults alone.
func buildTLSConfig(clientCert, clientKey string, insecure bool, logger *logrus.Logger) *tls.Config {
	if !insecure && (len(clientCert) == 0 || len(clientKey) == 0) {
		return nil
	}

	conf := &tls.Config{InsecureSkipVerify: insecure}

	if len(clientCert) == 0 || len(clientKey) == 0 {
		return conf
	}

	cert, err := tls.LoadX509KeyPair(clientCert, clientKey)
	if err != nil {
		// don't bail out, the agent is still able to talk to a server that
		// doesn't ask for a client cert
		logger.Errorln("buildTLSConfig() LoadX509KeyPair:", err)
		return conf
	}

	conf.Certificates = []tls.Certificate{cert}
	return conf
}

// clientTLSConfig returns the tls config for the rmm server, including the
// mTLS client cert when one is configured
func (a *Agent) clientTLSConfig() *tls.Config {
	return buildTLSConfig(a.ClientCert, a.ClientKey, a.Insecure, a.Logger)
}
