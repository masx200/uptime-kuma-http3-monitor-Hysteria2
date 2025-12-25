# Implementation Tasks

## Overview

This document outlines the step-by-step implementation plan for adding Uptime Kuma HTTP/3 monitoring capabilities to the `h3_fingerprint.go` tool.

## Task Breakdown

### Phase 1: Foundation (CLI and Configuration)

**Task 1.1: Add CLI flag parsing library**
- [ ] Add `flag` package to imports (std lib, no new dependencies)
- [ ] Define Config struct with fields for endpoints, interval, timeout, fingerprint-only mode
- [ ] Define EndpointConfig struct with name, target URL, SNI, push token, Kuma URL
- [ ] Implement `parseFlags()` function to parse and validate command-line arguments
- [ ] Add `--help` flag with usage documentation
- **Validation**: Run `./h3_monitor --help` and see all documented flags
- **Dependencies**: None

**Task 1.2: Implement flag pairing logic**
- [ ] Parse multiple `--target` flags into array
- [ ] Parse multiple `--sni` flags into array
- [ ] Parse multiple `--push-token` flags into array
- [ ] Implement pairing logic: index-based matching, reuse last token if fewer tokens than targets
- [ ] Validate that at least one target is specified
- [ ] Validate that URL schemes are https://
- [ ] Set defaults: interval=60s, timeout=10s, kuma-url=http://localhost:3001
- **Validation**: Run with 2 targets and 1 token, verify both monitors use same token
- **Dependencies**: Task 1.1

### Phase 2: HTTP/3 Client Refactoring

**Task 2.1: Extract HTTP/3 check logic into reusable function**
- [ ] Create `CheckHTTP3(target, sni string, timeout time.Duration) (*CheckResult, error)` function
- [ ] Move existing connection logic from `main()` into new function
- [ ] Add timeout wrapper using `time.After()` and `select` statement
- [ ] Measure response time using `time.Since()` before request
- [ ] Return `CheckResult` struct with success status, response time, fingerprint, error message
- [ ] Preserve original fingerprint extraction logic
- **Validation**: Unit test with mock HTTP/3 server returning success
- **Dependencies**: Task 1.1

**Task 2.2: Improve error handling in HTTP/3 client**
- [ ] Wrap connection errors with descriptive messages
- [ ] Distinguish between timeout, certificate, and network errors
- [ ] Log certificate changes (fingerprint mismatch) if previous state known
- [ ] Ensure context cancellation on timeout
- **Validation**: Unit test with unreachable target, verify error message quality
- **Dependencies**: Task 2.1

### Phase 3: Uptime Kuma Push Client

**Task 3.1: Implement push URL construction**
- [ ] Create `PushStatus(kumaURL, pushToken string, result *CheckResult) error` function
- [ ] Build push URL: `kumaURL + "/api/push/" + pushToken`
- [ ] Add query parameters: `status=up|down`, `msg=...`, `ping=<ms>`
- [ ] URL-encode the msg parameter to handle special characters
- [ ] Truncate msg to 250 characters if too long
- **Validation**: Manually verify URL format matches Uptime Kuma documentation
- **Dependencies**: Task 2.1

**Task 3.2: Implement push HTTP request**
- [ ] Create HTTP client with 5-second timeout for push requests
- [ ] Execute GET request to push URL
- [ ] Parse JSON response: `{"ok": true}` or `{"ok": false, "msg": "..."}`
- [ ] Handle 200 OK response (success)
- [ ] Handle 404 Not Found (log critical error, don't retry)
- [ ] Handle 5xx errors (log warning, retry once)
- [ ] Handle network errors (log warning, continue monitoring)
- **Validation**: Integration test with real Uptime Kuma instance
- **Dependencies**: Task 3.1

**Task 3.3: Add push retry logic**
- [ ] Implement single retry on 5xx errors
- [ ] Add 1-second delay before retry
- [ ] Log retry attempts with endpoint name
- [ ] Ensure retry doesn't block monitoring loop excessively
- **Validation**: Mock HTTP server returning 503, verify retry happens
- **Dependencies**: Task 3.2

### Phase 4: Monitoring Service Controller

**Task 4.1: Implement periodic monitoring loop**
- [ ] Create `monitorEndpoint(endpoint EndpointConfig, interval, timeout time.Duration, stopCh chan struct{})` function
- [ ] Create ticker with configured interval
- [ ] Loop until `stopCh` is closed
- [ ] Each tick: call `CheckHTTP3()`, then `PushStatus()`
- [ ] Log results with structured format: timestamp, level, endpoint name, status, ping, msg
- [ ] Handle and log panics with deferred recover()
- **Validation**: Manual test with 10-second interval, verify logs appear every 10s
- **Dependencies**: Task 2.1, Task 3.2

**Task 4.2: Implement concurrent monitoring**
- [ ] Create `StartMonitoring(config *Config)` function
- [ ] Create shared stop channel
- [ ] Launch goroutine for each endpoint using `monitorEndpoint()`
- [ ] Wait for SIGINT/SIGTERM signals using `signal.Notify()`
- [ ] On signal: close stop channel, wait for all goroutines to exit (using sync.WaitGroup)
- [ ] Log graceful shutdown message
- **Validation**: Run with 3 endpoints, send SIGINT, verify all monitors stop cleanly
- **Dependencies**: Task 4.1

**Task 4.3: Add graceful shutdown handling**
- [ ] Add `sync.WaitGroup` to track running goroutines
- [ ] Each goroutine calls `Done()` on exit
- [ ] Main goroutine calls `Wait()` after closing stop channel
- [ ] Add timeout to graceful shutdown (max 30 seconds to complete in-progress checks)
- **Validation**: Send SIGINT during active HTTP/3 check, verify check completes before exit
- **Dependencies**: Task 4.2

### Phase 5: Backward Compatibility

**Task 5.1: Implement fingerprint-only mode**
- [ ] Add `--fingerprint-only` flag to CLI parser
- [ ] When flag is set, run original logic: single check, print fingerprint, exit
- [ ] Skip monitoring loop and push integration
- [ ] Maintain exact same output format as original tool
- **Validation**: Run `./h3_monitor --fingerprint-only --target https://example.com`, verify output matches original
- **Dependencies**: Task 2.1

**Task 5.2: Add deprecation notice for old usage**
- [ ] If no flags provided (backward compatibility case), print warning
- [ ] Suggest using `--fingerprint-only` explicitly
- [ ] Continue with fingerprint-only mode
- **Validation**: Run `./h3_monitor` with no args, see deprecation notice
- **Dependencies**: Task 5.1

### Phase 6: Logging and Output

**Task 6.1: Implement structured logging**
- [ ] Define log level constants: INFO, WARN, ERROR, FATAL
- [ ] Create logging functions: `logInfo()`, `logWarn()`, `logError()`, `logFatal()`
- [ ] Format: `[timestamp] [level] endpoint="name" status="up/down" ping=245ms msg="..."`
- [ ] Use ISO 8601 timestamps
- [ ] Colorize output if terminal supports it (optional)
- **Validation**: Run monitoring, verify log format matches specification
- **Dependencies**: Task 4.1

**Task 6.2: Add startup logging**
- [ ] Log service start message with configuration (interval, timeout, endpoint count)
- [ ] Log each endpoint being monitored
- [ ] Log shutdown signal receipt
- [ ] Log final shutdown message with duration
- **Validation**: Start service, verify startup logs are clear and informative
- **Dependencies**: Task 4.2

### Phase 7: Documentation

**Task 7.1: Update README.md**
- [ ] Add "Monitoring Mode" section with usage examples
- [ ] Document all command-line flags
- [ ] Provide example commands for single and multi-endpoint monitoring
- [ ] Document Uptime Kuma push token setup
- [ ] Add troubleshooting section
- **Validation**: Peer review of README clarity
- **Dependencies**: All implementation tasks

**Task 7.2: Create example usage scripts**
- [ ] Create `examples/monitor-single.sh` - single endpoint example
- [ ] Create `examples/monitor-multiple.sh` - multi-endpoint example
- [ ] Create `examples/docker-compose.yml` - containerized deployment example
- [ ] Add comments explaining each flag
- **Validation**: Run each example script, verify it works
- **Dependencies**: Task 7.1

**Task 7.3: Add inline code documentation**
- [ ] Add Go doc comments to all exported functions
- [ ] Document struct fields with comments
- [ ] Add usage examples in function doc comments
- **Validation**: Run `godoc .`, verify documentation renders correctly
- **Dependencies**: All implementation tasks

### Phase 8: Testing

**Task 8.1: Write unit tests for config parsing**
- [ ] Test flag pairing with equal targets and tokens
- [ ] Test flag pairing with fewer tokens than targets
- [ ] Test default values for interval and timeout
- [ ] Test validation of missing required flags
- [ ] Test validation of invalid URL schemes
- **Validation**: All tests pass, coverage >80%
- **Dependencies**: Task 1.2

**Task 8.2: Write unit tests for HTTP/3 client**
- [ ] Mock successful HTTP/3 connection
- [ ] Mock connection timeout
- [ ] Mock certificate error
- [ ] Verify CheckResult structure is correct
- [ ] Verify response time measurement is accurate
- **Validation**: All tests pass
- **Dependencies**: Task 2.2

**Task 8.3: Write integration tests**
- [ ] Test against real HTTP/3 endpoint (e.g., https://cloudflare.com)
- [ ] Test with real Uptime Kuma instance (Docker container)
- [ ] Verify push appears in Uptime Kuma dashboard
- [ ] Test concurrent monitoring of 3 endpoints
- [ ] Test graceful shutdown with in-flight checks
- **Validation**: Manual testing confirms all scenarios work
- **Dependencies**: Task 4.3

### Phase 9: Build and Release

**Task 9.1: Update go.mod if needed**
- [ ] Review dependencies (currently only quic-go)
- [ ] Add any new dependencies if introduced (unlikely, std lib sufficient)
- [ ] Run `go mod tidy` to clean up
- **Validation**: `go build` succeeds without errors
- **Dependencies**: All implementation tasks

**Task 9.2: Update build script**
- [ ] If `build.sh` exists, update to build new binary name
- [ ] Add cross-compilation targets (linux-amd64, linux-arm64, darwin-amd64, windows-amd64)
- [ ] Add version information via ldflags
- **Validation**: Build succeeds for all target platforms
- **Dependencies**: Task 9.1

**Task 9.3: Create release artifacts**
- [ ] Build binaries for all platforms
- [ ] Create checksums.txt with SHA256 hashes
- [ ] Update README with download instructions
- [ ] Tag release in git (e.g., v2.0.0)
- **Validation**: Release artifacts are complete and tested
- **Dependencies**: Task 9.2

## Task Dependencies

```
Phase 1: Foundation
├─ Task 1.1 (CLI parsing)
└─ Task 1.2 (flag pairing) → depends on 1.1

Phase 2: HTTP/3 Client
├─ Task 2.1 (extract logic) → depends on 1.1
└─ Task 2.2 (error handling) → depends on 2.1

Phase 3: Push Client
├─ Task 3.1 (URL construction) → depends on 2.1
├─ Task 3.2 (HTTP request) → depends on 3.1
└─ Task 3.3 (retry logic) → depends on 3.2

Phase 4: Monitoring Controller
├─ Task 4.1 (monitoring loop) → depends on 2.1, 3.2
├─ Task 4.2 (concurrent monitoring) → depends on 4.1
└─ Task 4.3 (graceful shutdown) → depends on 4.2

Phase 5: Backward Compatibility
├─ Task 5.1 (fingerprint-only mode) → depends on 2.1
└─ Task 5.2 (deprecation notice) → depends on 5.1

Phase 6: Logging
├─ Task 6.1 (structured logging) → depends on 4.1
└─ Task 6.2 (startup logging) → depends on 4.2

Phase 7: Documentation
├─ Task 7.1 (update README) → depends on all implementation
├─ Task 7.2 (example scripts) → depends on 7.1
└─ Task 7.3 (inline docs) → depends on all implementation

Phase 8: Testing
├─ Task 8.1 (config tests) → depends on 1.2
├─ Task 8.2 (client tests) → depends on 2.2
└─ Task 8.3 (integration tests) → depends on 4.3

Phase 9: Build and Release
├─ Task 9.1 (update go.mod) → depends on all implementation
├─ Task 9.2 (update build script) → depends on 9.1
└─ Task 9.3 (release artifacts) → depends on 9.2
```

## Parallelizable Work

The following tasks can be done in parallel by multiple developers:

**Group A**: Phase 2 (HTTP/3 Client) and Phase 3 (Push Client) can be developed in parallel after Phase 1 is complete, as they operate on different concerns.

**Group B**: Phase 6 (Logging) can be developed alongside Phase 4 (Monitoring Controller) by a different developer, as logging is a cross-cutting concern.

**Group C**: Phase 7 (Documentation) tasks can be split among multiple writers, with one person on README, one on examples, and one on inline docs.

**Group D**: Phase 8 (Testing) unit tests can be written in parallel with implementation (TDD approach), while integration tests must wait until implementation is complete.

## Estimated Timeline

- **Phase 1-3** (Core functionality): 3-4 hours
- **Phase 4-6** (Monitoring service): 2-3 hours
- **Phase 7** (Documentation): 1-2 hours
- **Phase 8** (Testing): 2-3 hours
- **Phase 9** (Build and release): 1 hour

**Total**: 9-13 hours for a single developer
**With parallelization** (2-3 developers): 4-6 hours

## Definition of Done

Each task is considered complete when:
- [ ] Code is written and follows Go best practices
- [ ] Unit tests exist and pass (if applicable)
- [ ] Code is reviewed by at least one other person
- [ ] Documentation is updated (if applicable)
- [ ] Manual testing confirms the feature works as specified
- [ ] No regressions in existing functionality
