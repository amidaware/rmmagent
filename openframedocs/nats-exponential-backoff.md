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
| 6+ | 5m (max) | 3m 30s – 5m* |

\* Delay is clamped to a minimum of 5s after jitter is applied.

## Implementation

The NATS options are split into two separate methods:

- `setupDefaultNatsOptions()` — original behavior with fixed `ReconnectWait` (random 2–8s)
- `setupOpenframeNatsOptions()` — exponential backoff via `nats.CustomReconnectDelay`

The backoff delay is computed in `natsReconnectDelay(attempts int) time.Duration`:

```go
delay = baseDelay * 2^attempts       // exponential growth
delay = min(delay, maxDelay)          // cap at 5 minutes
delay += delay * jitter * random()    // add ±30% jitter
delay = max(delay, baseDelay)         // floor at 1 second
```

## Logging

Reconnect events are logged at **Info** level in Openframe mode:

```
INFO  NATS reconnect attempt 0, waiting 1.2s
INFO  NATS disconnected: <error>
INFO  NATS reconnected
```

## Files changed

- `agent/agent.go` — Added `natsReconnectDelay()`, `setupOpenframeNatsOptions()`, `setupDefaultNatsOptions()`
