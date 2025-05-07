package agent

import (
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

type OpenframeConnectionManager struct {
	rClient  *resty.Client
	natsConn *nats.Conn
	logger   *logrus.Logger
}

func NewOpenframeConnectionManager(rClient *resty.Client, logger *logrus.Logger) *OpenframeConnectionManager {
	return &OpenframeConnectionManager{
		rClient:  rClient,
		natsConn: nil,
		logger:   logger,
	}
}

func (cm *OpenframeConnectionManager) SetNatsConnection(natsConn *nats.Conn) {
	cm.natsConn = natsConn
	cm.logger.Debugln("NATS connection set in connection manager")
}

func (cm *OpenframeConnectionManager) UpdateRestClient(token string) {
	cm.rClient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", token))
	cm.logger.Debugln("Rest token updated")
}

func (cm *OpenframeConnectionManager) UpdateNatsConnection(token string) {
	if cm.natsConn == nil {
		cm.logger.Debugln("Nats connection is nil, skipping reconnection")
		return
	}

	cm.natsConn.Opts.ProxyPath = fmt.Sprintf(wsProxyPathTemplate, token)
	cm.logger.Debugln("Updated nats options with new token")
	cm.logger.Debugln("Nats options: ", cm.natsConn.Opts)
	cm.logger.Debugln("Force reconnecting nats")
	cm.natsConn.ForceReconnect()
	cm.logger.Debugln("Forced nats reconnection")
}
