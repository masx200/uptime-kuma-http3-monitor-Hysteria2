# Design: Uptime Kuma HTTP/3 Monitoring Service

## Overview

This document describes the architectural design for transforming
`h3_fingerprint.go` into a continuous HTTP/3 monitoring service with Uptime Kuma
push integration.

## Architecture

### Component Structure

```
┌─────────────────────────────────────────────────────────────┐
│                     CLI Entry Point                         │
│              (parse flags, validate config)                  │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│              Monitor Service Controller                      │
│         - Spawns goroutines per endpoint                     │
│         - Handles shutdown signals                           │
│         - Coordinates monitoring loop                        │
└──────────────────────┬──────────────────────────────────────┘
                       │
        ┌──────────────┼──────────────┐
        ▼              ▼              ▼
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│  Endpoint 1 │ │  Endpoint 2 │ │  Endpoint N │
│  Monitor    │ │  Monitor    │ │  Monitor    │
└──────┬──────┘ └──────┬──────┘ └──────┬──────┘
       │               │               │
       └───────────────┼───────────────┘
                       │
        ┌──────────────┴──────────────┐
        ▼                             ▼
┌──────────────────┐         ┌──────────────────┐
│  HTTP/3 Client   │         │  Uptime Kuma     │
│  - TLS Config    │         │  Push Client     │
│  - QUIC Transport│         │  - HTTP Request  │
│  - Fingerprint   │         │  - Status Report │
└──────────────────┘         └──────────────────┘
```

### Core Modules

#### 1. Configuration Module

**Purpose**: Parse and validate command-line arguments

**Structures**:

```go
type Config struct {
    Endpoints []EndpointConfig
    Interval  time.Duration
    Timeout   time.Duration
    FingerprintOnly bool
}

type EndpointConfig struct {
    Name      string
    TargetURL string
    SNIServerName string
    PushToken string
    KumaURL   string
}
```

**Command-Line Flags**:

- `--target <url>`: Target HTTP/3 endpoint (can be specified multiple times)
- `--sni <servername>`: SNI server name for TLS (paired with target)
- `--push-token <token>`: Uptime Kuma push token (paired with target)
- `--kuma-url <url>`: Uptime Kuma base URL (default: http://localhost:3001)
- `--interval <seconds>`: Monitoring interval (default: 60)
- `--timeout <seconds>`: HTTP/3 connection timeout (default: 10)
- `--fingerprint-only`: Run once and exit, print fingerprint only

**Pairing Logic**: Flags are paired by index. If 3 targets are specified with 3
push tokens, they map 1-to-1. If fewer push tokens than targets, last token is
reused.

#### 2. HTTP/3 Client Module

**Purpose**: Encapsulate HTTP/3 connection and health check logic

**Key Functions**:

```go
func CheckHTTP3(target, sni string, timeout time.Duration) (*CheckResult, error) {
    // - Create HTTP/3 RoundTripper with TLS config
    // - Execute HEAD request with timeout
    // - Measure response time
    // - Extract certificate fingerprint
    // - Return result or error
}

type CheckResult struct {
    Success         bool
    ResponseTime    time.Duration
    CertificateFingerprint string
    ErrorMsg        string
}
```

**Error Handling**:

- Connection timeout → Return error with timeout message
- Certificate validation error → Return error with cert details
- Network error → Return error with underlying message

#### 3. Uptime Kuma Push Client Module

**Purpose**: Send status updates to Uptime Kuma push endpoint

**Key Functions**:

```go
func PushStatus(kumaURL, pushToken string, result *CheckResult) error {
    // - Build push URL: /api/push/{pushToken}
    // - Set query params: status, msg, ping
    // - Execute HTTP GET/POST
    // - Handle response: 200 OK, 404 Not Found, 5xx errors
}
```

**Status Mapping**:

- `result.Success == true` → `status=up`, `ping=<ms>`, `msg=OK`
- `result.Success == false` → `status=down`, `msg=<error details>`

**Retry Logic**:

- On 5xx errors: Log and retry once
- On 404 errors: Log critical error (invalid token or monitor disabled)
- On network errors: Log and continue (don't halt monitoring)

#### 4. Monitor Service Controller

**Purpose**: Orchestrate monitoring loop and manage goroutines

**Key Functions**:

```go
func StartMonitoring(config *Config) {
    // - Create channel for shutdown signals
    // - Launch goroutine per endpoint
    // - Wait for SIGINT/SIGTERM
    // - Graceful shutdown: in-progress checks complete, then exit
}

func monitorEndpoint(endpoint EndpointConfig, interval, timeout time.Duration, stopCh chan struct{}) {
    // - Ticker for periodic checks
    // - Loop until stopCh closed
    // - Each tick: run CheckHTTP3() then PushStatus()
    // - Log results with structured logging
}
```

**Concurrency Model**:

- Each endpoint monitored in separate goroutine
- Shared stop channel for coordinated shutdown
- No shared state between monitors (thread-safe by design)

## Data Flow

### Normal Operation (Success Case)

```
1. Timer fires for Endpoint A
   ↓
2. HTTP3.CheckHTTP3("https://target:port", "sni", 10s)
   ↓
3. Establish QUIC connection → TLS handshake → HTTP HEAD request
   ↓
4. Measure response time: 245ms
   ↓
5. Extract certificate fingerprint: abc123...
   ↓
6. Return CheckResult{Success: true, ResponseTime: 245ms}
   ↓
7. UptimeKuma.PushStatus(kumaURL, tokenXYZ, result)
   ↓
8. Build URL: http://kuma:3001/api/push/tokenXYZ?status=up&ping=245&msg=OK
   ↓
9. Execute HTTP GET → 200 OK {"ok": true}
   ↓
10. Log: "Endpoint A: UP (245ms)"
```

### Failure Case

```
1. Timer fires for Endpoint B
   ↓
2. HTTP3.CheckHTTP3("https://target:port", "sni", 10s)
   ↓
3. QUIC connection timeout after 10s
   ↓
4. Return error: "dial timeout: no connection established"
   ↓
5. Build CheckResult{Success: false, ErrorMsg: "..."}
   ↓
6. UptimeKuma.PushStatus(kumaURL, tokenABC, result)
   ↓
7. Build URL: http://kuma:3001/api/push/tokenABC?status=down&msg=dial+timeout...
   ↓
8. Execute HTTP GET → 200 OK {"ok": true}
   ↓
9. Log: "Endpoint B: DOWN - dial timeout: no connection established"
```

## Technical Decisions

### 1. Sequential vs Concurrent Monitoring

**Decision**: Concurrent (one goroutine per endpoint)

**Rationale**:

- Endpoint failures should not block monitoring of other endpoints
- Allows independent intervals per endpoint (future enhancement)
- Go's goroutine model makes this simple and efficient

**Trade-offs**:

- Slightly more complex code vs sequential
- Risk of overwhelming network if too many endpoints
  - Mitigation: Document reasonable limits (e.g., max 10 endpoints)

### 2. Connection Reuse vs New Connection Per Check

**Decision**: New HTTP/3 connection for each check

**Rationale**:

- QUIC connections are stateful and may timeout between checks
- Simpler implementation, no connection pool management
- Better simulates real client behavior (fresh connection each time)

**Trade-offs**:

- Slightly higher overhead vs connection reuse
- More realistic monitoring of actual handshake performance

### 3. Blocking vs Non-Blocking Push

**Decision**: Blocking push (wait for response before next check)

**Rationale**:

- Simpler error handling and logging
- Prevents request buildup if Uptime Kuma is slow
- Monitoring interval is typically long enough (60s) that push latency is
  negligible

**Trade-offs**:

- Push failure delays next check slightly
  - Mitigation: Push timeout of 5s separate from check timeout

### 4. Flag-Based vs Config File Configuration

**Decision**: Flag-based (command-line arguments)

**Rationale**:

- User explicitly requested command-line configuration
- Simpler for containerized environments (Docker, Kubernetes)
- Easier to document and script
- No file parsing dependencies

**Trade-offs**:

- Longer command lines for multiple endpoints
  - Mitigation: Support environment variable fallback in future

### 5. Timeout Strategy

**Decision**: Separate timeouts for HTTP/3 check (10s) and push (5s)

**Rationale**:

- HTTP/3 may legitimately take longer due to handshake
- Push should fail fast to not delay monitoring
- Independent timeouts allow fine-tuning

**Configuration**:

```go
CheckTimeout: 10s (configurable via --timeout)
PushTimeout: 5s (internal constant)
```

## Error Handling Strategy

### HTTP/3 Check Errors

| Error Type          | Handling                              | User Action                               |
| ------------------- | ------------------------------------- | ----------------------------------------- |
| Connection timeout  | Log, push "down" with timeout message | Check target endpoint availability        |
| Certificate error   | Log, push "down" with cert error      | Verify TLS config, check for cert changes |
| DNS failure         | Log, push "down" with DNS error       | Check DNS resolution                      |
| Network unreachable | Log, push "down" with network error   | Check network connectivity                |

### Uptime Kuma Push Errors

| Error Type       | Handling                          | User Action                              |
| ---------------- | --------------------------------- | ---------------------------------------- |
| 404 Not Found    | Log critical, continue monitoring | Verify push token and monitor activation |
| 5xx Server Error | Log warning, retry once           | Check Uptime Kuma instance health        |
| Network error    | Log warning, continue monitoring  | Check connectivity to Uptime Kuma        |
| Timeout (5s)     | Log warning, continue monitoring  | Check Uptime Kuma responsiveness         |

### Panic Recovery

Each goroutine has deferred recover():

```go
defer func() {
    if r := recover(); r != nil {
        log.Printf("Panic in monitor for %s: %v", endpoint.Name, r)
    }
}()
```

## Logging Strategy

### Log Levels

- **INFO**: Successful checks, service start/stop
- **WARN**: Push failures (non-critical), retries
- **ERROR**: HTTP/3 check failures (but push succeeded)
- **FATAL**: Configuration errors, startup failures

### Log Format

Structured logging with consistent fields:

```
[timestamp] [level] endpoint="name" status="up/down" ping=245ms msg="..."
```

### Example Logs

```
2025-12-25T10:00:00Z [INFO] Starting HTTP/3 monitoring service (interval=60s)
2025-12-25T10:00:00Z [INFO] Spawning monitor for endpoint "Production"
2025-12-25T10:00:01Z [INFO] endpoint="Production" status="up" ping=245ms msg="OK"
2025-12-25T10:01:01Z [INFO] endpoint="Production" status="up" ping=238ms msg="OK"
2025-12-25T10:02:01Z [ERROR] endpoint="Production" status="down" msg="dial timeout: no connection established"
2025-12-25T10:02:01Z [WARN] endpoint="Production" push failed: 503 Service Unavailable, retrying...
2025-12-25T10:02:02Z [INFO] endpoint="Production" push succeeded on retry
```

## Testing Strategy

### Unit Tests

1. **Config Parsing**: Validate flag pairing and defaults
2. **HTTP3 Client**: Mock responses, test success/failure paths
3. **Push Client**: Mock HTTP server, test URL construction and error handling
4. **Status Mapping**: Verify correct status/msg/pig values

### Integration Tests

1. **Real HTTP/3 Endpoint**: Test against known-good HTTP/3 server (e.g.,
   cloudflare.com)
2. **Real Uptime Kuma**: Spin up test Uptime Kuma instance, verify push
   registration
3. **Concurrent Monitors**: Run multiple endpoints, verify no race conditions
4. **Graceful Shutdown**: Send SIGTERM, verify in-progress checks complete

### Manual Testing Checklist

- [x] Single endpoint monitoring works
- [x] Multiple endpoints with independent tokens work
- [x] Connection timeout triggers "down" status
- [x] Invalid push token logs 404 error
- [x] Uptime Kuma unavailable triggers retry
- [x] SIGINT causes graceful shutdown
- [x] `--fingerprint-only` mode preserves original behavior
- [x] Response time accurately measured and reported

## Security Considerations

1. **Push Token Exposure**: Tokens in command-line arguments visible in `ps`
   - Mitigation: Document security implications, recommend environment variables
     in future
2. **InsecureSkipVerify**: Current code skips TLS verification
   - Mitigation: Preserve existing behavior, document risk
3. **No Authentication**: Uptime Kuma push endpoint relies on token secrecy
   - Mitigation: Document token protection requirements
4. **Logging Sensitive Data**: Avoid logging push tokens or full URLs
   - Implementation: Log token prefix only (e.g., "abc...xyz")

## Performance Considerations

### Resource Usage

- **Memory**: ~50MB base + ~5MB per concurrent goroutine
- **CPU**: Minimal during idle, spike during HTTP/3 handshake
- **Network**: 1 HTTP/3 connection + 1 HTTP push per endpoint per interval

### Scalability Limits

- **Max Endpoints**: 10 (practical limit based on goroutine overhead)
- **Min Interval**: 10 seconds (below this may overwhelm targets)
- **Max Interval**: 86400 seconds (1 day, reasonable upper bound)

### Optimization Opportunities

1. **Connection Pooling**: Reuse HTTP/3 connections (adds complexity)
2. **Batch Pushes**: Send multiple endpoint updates in one request (requires
   Uptime Kuma API changes)
3. **Adaptive Intervals**: Increase interval on repeated failures (future
   enhancement)

## Future Enhancements

1. **Environment Variable Support**: Allow configuration via env vars
2. **Configuration File**: YAML/JSON config for complex deployments
3. **Prometheus Metrics**: Expose metrics for scraping
4. **Web UI**: Simple dashboard for endpoint status
5. **Historical Data**: Store check results locally for trend analysis
6. **Smart Retry**: Exponential backoff on repeated failures
7. **Health Check Endpoint**: HTTP endpoint for service health
