package network

import (
	"net"
	"time"

	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/go-resty/resty/v2"
)

// PublicIP returns the agent's public ip
// Tries 3 times before giving up
func PublicIP(proxy string) string {
	client := resty.New()
	client.SetTimeout(4 * time.Second)
	if len(proxy) > 0 {
		client.SetProxy(proxy)
	}

	urls := []string{"https://icanhazip.tacticalrmm.io/", "https://icanhazip.com", "https://ifconfig.co/ip"}
	ip := "error"
	for _, url := range urls {
		r, err := client.R().Get(url)
		if err != nil {
			continue
		}

		ip = utils.StripAll(r.String())
		if !IsValidIP(ip) {
			continue
		}

		v4 := net.ParseIP(ip)
		if v4.To4() == nil {
			r1, err := client.R().Get("https://ifconfig.me/ip")
			if err != nil {
				return ip
			}

			ipv4 := utils.StripAll(r1.String())
			if !IsValidIP(ipv4) {
				continue
			}

			return ipv4
		}

		break
	}

	return ip
}

// IsValidIP checks for a valid ipv4 or ipv6
func IsValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}
