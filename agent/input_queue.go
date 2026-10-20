/*
Copyright 2026 Amidaware Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"runtime"
	"sync"
	"time"
)

// each session gets its own line for input, handled one at a time, so keys and
// clicks show up in the same order they were typed. If the line backs up, we drop
// extra input instead of making everything else wait.

type inputLogger interface {
	Debugln(args ...interface{})
}

const inputQueueDepth = 256

const inputWorkerIdle = 30 * time.Second

type inputDispatcher struct {
	mu      sync.Mutex
	workers map[string]chan string
	idle    time.Duration
}

func (d *inputDispatcher) enqueue(sessionID, payload, what string, feed func(sessionID, payload string) error, logger inputLogger) {
	d.mu.Lock()
	defer d.mu.Unlock()

	ch, ok := d.workers[sessionID]
	if !ok {
		if d.workers == nil {
			d.workers = map[string]chan string{}
		}
		ch = make(chan string, inputQueueDepth)
		d.workers[sessionID] = ch
		idle := d.idle
		if idle <= 0 {
			idle = inputWorkerIdle
		}
		go d.work(sessionID, ch, idle, what, feed, logger)
	}

	select {
	case ch <- payload:
	default:
		logger.Debugln(what + ": dispatch queue full, dropping batch")
	}
}

func (d *inputDispatcher) work(sessionID string, ch chan string, idleFor time.Duration, what string, feed func(sessionID, payload string) error, logger inputLogger) {
	idle := time.NewTimer(idleFor)
	defer idle.Stop()

	for {
		select {
		case payload := <-ch:
			if err := feed(sessionID, payload); err != nil {
				logger.Debugln(what+":", err)
			}
		case <-idle.C:
			d.mu.Lock()
			if len(ch) == 0 {
				delete(d.workers, sessionID)
				d.mu.Unlock()
				return
			}
			d.mu.Unlock()
		}

		if !idle.Stop() {
			select {
			case <-idle.C:
			default:
			}
		}
		idle.Reset(idleFor)
	}
}

var terminalInputDispatch inputDispatcher

// QueueTerminalInput dispatches one terminal input chunk in arrival order
func QueueTerminalInput(a *Agent, sessionID, data string) {
	terminalInputDispatch.enqueue(sessionID, data, "terminal_input",
		func(id, payload string) error {
			if runtime.GOOS == "windows" {
				return FeedTerminalInputWindows(id, payload)
			}
			return a.FeedTerminalInput(id, payload)
		}, a.Logger)
}
