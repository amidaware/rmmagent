//go:build !extra

/*
Copyright 2026 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import "time"

func (a *Agent) extraTicker() *time.Ticker { return nil }
func (a *Agent) runExtra()                 {}
