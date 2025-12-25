# Proposal: Add Uptime Kuma HTTP/3 Monitoring

## Change ID
`add-uptime-kuma-http3-monitor`

## Overview

Transform the existing `h3_fingerprint.go` utility from a one-time certificate fingerprint extraction tool into a continuous HTTP/3 monitoring service that pushes status updates to Uptime Kuma via its push API endpoint.

## Problem Statement

The current `h3_fingerprint.go` tool is a single-run utility that:
- Connects to one hardcoded HTTP/3 endpoint (51.83.6.7:20143)
- Extracts and displays TLS certificate SHA256 fingerprint
- Exits after one check
- Provides no monitoring, alerting, or status persistence

For production monitoring of HTTP/3 services, users need:
- Continuous health checking with configurable intervals
- Integration with monitoring platforms (Uptime Kuma)
- Support for monitoring multiple HTTP/3 endpoints
- Automated status reporting with error details
- Graceful error handling and recovery

## Proposed Solution

Refactor `h3_fingerprint.go` into a daemon-style monitoring service with the following capabilities:

1. **HTTP/3 Health Monitoring**: Periodically connect to configured HTTP/3 endpoints and verify connectivity
2. **Uptime Kuma Push Integration**: Send status updates via Uptime Kuma's `/api/push/<pushToken>` endpoint
3. **Multi-Endpoint Support**: Configure and monitor multiple HTTP/3 targets with independent push tokens
4. **Command-Line Configuration**: Support all configuration via command-line arguments
5. **Detailed Error Reporting**: Include error messages in push notifications for debugging

## Scope

### In Scope
- Refactoring existing HTTP/3 connection logic to support repeated checks
- Adding Uptime Kuma push API client functionality
- Implementing periodic monitoring loop with configurable intervals
- Adding command-line argument parsing for configuration
- Supporting multiple HTTP/3 endpoints with individual configurations
- Including error details in status push messages
- Measuring and reporting response time (ping) to Uptime Kuma

### Out of Scope
- Web UI or dashboard for configuration
- Database storage of historical monitoring data
- Alert delivery channels beyond Uptime Kuma (email, SMS, etc.)
- Advanced retry logic with exponential backoff
- Concurrent monitoring of multiple endpoints (sequential is acceptable)

## Success Criteria

1. Service runs continuously and performs HTTP/3 checks at configured intervals
2. Successful connections push `status=up` with response time to Uptime Kuma
3. Failed connections push `status=down` with error message in `msg` parameter
4. Multiple endpoints can be monitored simultaneously with independent push tokens
5. Configuration is entirely via command-line arguments (no hardcoded values)
6. Process handles network errors gracefully and continues monitoring
7. Service responds to termination signals (SIGINT, SIGTERM) for graceful shutdown

## Dependencies

### External Dependencies
- Uptime Kuma instance with push monitoring configured
- HTTP/3-enabled target endpoints
- Network connectivity to both Uptime Kuma and target endpoints

### Internal Dependencies
- Existing `h3_fingerprint.go` HTTP/3 connection logic
- `github.com/quic-go/quic-go/http3` package (already in go.mod)

## Risks and Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Uptime Kuma endpoint unavailable | Medium | Buffer last status locally, retry with backoff |
| HTTP/3 endpoint certificate changes | Low | Log fingerprint changes, continue monitoring |
| Memory leak in long-running process | Medium | Implement connection pooling, periodic testing |
| Too frequent polling overwhelms targets | Low | Default to reasonable interval (e.g., 60s), document best practices |

## Open Questions

1. **Default monitoring interval**: Should we default to 60 seconds? Allow override via flag?
   - Recommendation: Default 60s, configurable via `--interval` flag
2. **Timeout per HTTP/3 check**: What's a reasonable timeout?
   - Recommendation: 10 seconds default, configurable via `--timeout` flag
3. **Handling all endpoints down**: Should service exit or continue retrying?
   - Recommendation: Continue retrying indefinitely, log errors
4. **Multiple push endpoints for same target**: Should we support pushing to multiple Uptime Kuma instances?
   - Recommendation: Out of scope for initial implementation

## Related Changes

None - this is a new feature addition

## Migration Notes

This is a new monitoring tool. The original `h3_fingerprint.go` functionality (certificate extraction) will be preserved as a one-shot mode via a `--fingerprint-only` flag for backward compatibility.

## Timeline Estimate

- Implementation: 2-3 hours
- Testing: 1 hour
- Documentation: 30 minutes
