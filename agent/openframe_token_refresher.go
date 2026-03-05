package agent

import (
	"fmt"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

type OpenframeTokenRefresher struct {
	a                 *Agent
	connectionManager *OpenframeConnectionManager
	tokenExtractor    *OpenframeTokenExtractor
	cron              *cron.Cron
	logger            *logrus.Logger
	extractErrCount   int
}

func NewOpenframeTokenRefresher(
	a *Agent,
	connectionManager *OpenframeConnectionManager,
	tokenExtractor *OpenframeTokenExtractor,
	logger *logrus.Logger,
) *OpenframeTokenRefresher {
	return &OpenframeTokenRefresher{
		a:                 a,
		connectionManager: connectionManager,
		tokenExtractor:    tokenExtractor,
		cron:              cron.New(cron.WithSeconds()),
		logger:            logger,
	}
}

func (tr *OpenframeTokenRefresher) Start() error {
	_, err := tr.cron.AddFunc("*/5 * * * * *", tr.refreshToken)
	if err != nil {
		return fmt.Errorf("failed to schedule token refresh job: %v", err)
	}
	tr.cron.Start()
	tr.logger.Infoln("Token refresh job started")
	return nil
}

func (tr *OpenframeTokenRefresher) Stop() {
	if tr.cron != nil {
		tr.cron.Stop()
		tr.logger.Infoln("Token refresh job stopped")
	}
}

func (tr *OpenframeTokenRefresher) refreshToken() {
	token, err := tr.tokenExtractor.ExtractToken()
	if err != nil {
		tr.extractErrCount++
		if tr.extractErrCount % openframeTokenRefreshErrorLogInterval == 1 {
			tr.logger.Errorf("Error extracting token: %v", err)
		}
		return
	}
	tr.extractErrCount = 0

	if tr.a.OpenframeAccessToken == token {
		return
	}

	tr.a.OpenframeAccessToken = token
	tr.logger.Infoln("Openframe token refreshed")

	tr.connectionManager.UpdateRestClient(token)
	tr.connectionManager.UpdateNatsConnection(token)
}
