# NATS Exponential Backoff

## Overview

In Openframe mode, the agent uses exponential backoff with jitter when reconnecting to the NATS server. This prevents thundering herd problems when many agents lose connection simultaneously (e.g. during server restarts or network issues).

In default (non-Openframe) mode, the original fixed random delay of 2–8 seconds is preserved.

## Backoff parameters

| Parameter | Value |
|-----------|-------|
| Base delay | 5s |
| Max delay | 5m |
| Jitter | ±30% |
| Max attempts cap | 6 |
| Max reconnects | unlimited |

## Backoff table

| Attempt | Delay (without jitter) | Delay range (with ±30% jitter) |
|---------|------------------------|--------------------------------|
| 0 | 5s | 3.5s – 6.5s |
| 1 | 10s | 7s – 13s |
| 2 | 20s | 14s – 26s |
| 3 | 40s | 28s – 52s |
| 4 | 1m 20s | 56s – 1m 44s |
| 5 | 2m 40s | 1m 52s – 3m 28s |
| 6+ | 5m (max) | ~3m 30s – ~6m 30s |

Attempts 7 and above use the same delay as attempt 6 (effAttempts capped at 6).

## Implementation

In Openframe mode, `setupNatsOptions()` uses `nats.CustomReconnectDelay(a.natsReconnectDelay)`. The delay is computed in `natsReconnectDelay(attempts int) time.Duration`:

```go
effAttempts = min(attempts, natsBackoffMaxAttemptsCap)   // cap at 6
delay = baseDelay * 2^effAttempts                        // exponential growth
delay = min(delay, maxDelay)                             // cap at 5 minutes
delay += delay * jitter * (random in [-1, 1])            // add ±30% jitter
```

The attempts cap avoids overflow: at attempt 31+, `5*2^attempts` nanoseconds overflows int64, which would produce garbage durations (e.g. "2385479h" or 5s) and cause a reconnect storm.

## Logging

Reconnect events are logged at **Info** level in Openframe mode:

```
INFO  NATS reconnect attempt 0, waiting 5.2s
INFO  NATS disconnected: <error>
INFO  NATS reconnected
```

## Files

- `agent/agent.go` — constants `natsBackoffBase`, `natsBackoffMax`, `natsBackoffJitter`, `natsBackoffMaxAttemptsCap`; `natsReconnectDelay()`; `setupNatsOptions()` uses `CustomReconnectDelay` when `OpenframeMode` is true.
