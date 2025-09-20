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
	tr.logger.Println("Scheduling token refresh job")
	_, err := tr.cron.AddFunc("*/5 * * * * *", tr.refreshToken)
	if err != nil {
		return fmt.Errorf("failed to schedule token refresh job: %v", err)
	}
	tr.cron.Start()
	tr.logger.Println("Token refresh job started")
	return nil
}

func (tr *OpenframeTokenRefresher) Stop() {
	if tr.cron != nil {
		tr.logger.Println("Stopping token refresh job")
		tr.cron.Stop()
		tr.logger.Println("Token refresh job stopped")
	}
}

func (tr *OpenframeTokenRefresher) refreshToken() {
	tr.logger.Println("Refreshing token")

	token, err := tr.tokenExtractor.ExtractToken()
	if err != nil {
		tr.logger.Printf("Error extracting token: %v", err)
		return
	}

	if tr.a.OpenframeAccessToken == token {
		tr.logger.Debugln("Openframe token is the same, skipping refresh")
		return
	}

	tr.a.OpenframeAccessToken = token
	tr.logger.Debugln("Openframe token updated")

	tr.connectionManager.UpdateRestClient(token)
	tr.connectionManager.UpdateNatsConnection(token)
}
