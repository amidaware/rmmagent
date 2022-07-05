package api

import (
	"fmt"
	"time"

	"github.com/amidaware/rmmagent/agent/tactical/config"
	"github.com/go-resty/resty/v2"
)

var restyC *resty.Client

func init() {
	ac := config.NewAgentConfig()
	headers := make(map[string]string)
	if len(ac.Token) > 0 {
		headers["Content-Type"] = "application/json"
		headers["Authorization"] = fmt.Sprintf("Token %s", ac.Token)
	}

	restyC = resty.New()
	restyC.SetBaseURL(ac.BaseURL)
	restyC.SetCloseConnection(true)
	restyC.SetHeaders(headers)
	restyC.SetTimeout(15 * time.Second)

	if len(ac.Proxy) > 0 {
		restyC.SetProxy(ac.Proxy)
	}

	if len(ac.Cert) > 0 {
		restyC.SetRootCertificate(ac.Cert)
	}
}

func PostPayload(payload interface{}, url string) error {
	fmt.Println(restyC.BaseURL)
	_, err := restyC.R().SetBody(payload).Post(url)
	if err != nil {
		return err
	}

	return nil
}

func GetResult(result interface{}, url string) (*resty.Response, error) {
	r, err := restyC.R().SetResult(result).Get(url)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func Get(url string) (*resty.Response, error) {
	r, err := restyC.R().Get(url)
	if err != nil {
		return nil, err
	}

	return r, nil
}

func Patch(payload interface{}, url string) error {
	_, err := restyC.R().SetBody(payload).Patch(url)
	if err != nil {
		return err
	}

	return nil
}

func Put(payload interface{}, url string) error {
	_, err := restyC.R().SetBody(payload).Put(url)
	if err != nil {
		return err
	}

	return nil
}
