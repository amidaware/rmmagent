package agent

import (
	"fmt"
	"log"

	"github.com/robfig/cron/v3"
)

type OpenframeTokenRefresher struct {
	a              *Agent
	cron           *cron.Cron
	tokenExtractor *OpenframeTokenExtractor
}

func NewOpenframeTokenRefresher(a *Agent, tokenExtractor *OpenframeTokenExtractor) *OpenframeTokenRefresher {
	return &OpenframeTokenRefresher{
		a:              a,
		cron:           cron.New(),
		tokenExtractor: tokenExtractor,
	}
}

func (tr *OpenframeTokenRefresher) Start() error {
	// Schedule the job to run every minute
	log.Println("Scheduling token refresh job")
	_, err := tr.cron.AddFunc("* * * * *", tr.refreshToken)
	if err != nil {
		return fmt.Errorf("failed to schedule token refresh job: %v", err)
	}
	tr.cron.Start()
	log.Println("Token refresh job started")
	return nil
}

func (tr *OpenframeTokenRefresher) Stop() {
	if tr.cron != nil {
		log.Println("Stopping token refresh job")
		tr.cron.Stop()
		log.Println("Token refresh job stopped")
	}
}

func (tr *OpenframeTokenRefresher) refreshToken() {
	log.Println("Refreshing token")

	token, err := tr.tokenExtractor.ExtractToken()
	if err != nil {
		log.Printf("Error extracting token: %v", err)
		return
	}

	log.Printf("New token: %s", token)

	a := tr.a;

	if a.OpenframeAccessToken != token {
		log.Println("Openframe token changed, updating connections...")
		a.OpenframeAccessToken = token
		a.Logger.Debugln("Openframe token updated")
		a.rClient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", token))
		a.Logger.Debugln("Rest token updated")
		a.Logger.Debugln("Checking NATS connection status")

		a.NatsConn.Opts.ProxyPath = fmt.Sprintf(wsProxyPathTemplate, token)
		a.Logger.Debugln("Updated nats options with new token")
		a.Logger.Debugln("Nats options: ", tr.a.NatsConn.Opts)
		a.Logger.Debugln("Force reconnecting nats")
		a.NatsConn.ForceReconnect()
		a.Logger.Debugln("Forced nats reconnection")
	} else {
		a.Logger.Debugln("Openframe token is the same, skipping refresh")
	}
}
