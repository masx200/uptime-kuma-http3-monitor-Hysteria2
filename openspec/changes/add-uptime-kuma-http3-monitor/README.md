# OpenSpec Proposal Summary

## Change ID: `add-uptime-kuma-http3-monitor`

**Status**: ✅ Validated and Ready for Review **Validation**:
`openspec validate add-uptime-kuma-http3-monitor --strict` ✓ PASSED

## Quick Overview

This proposal transforms the existing `h3_fingerprint.go` utility from a
one-time certificate fingerprint extraction tool into a continuous HTTP/3
monitoring service that pushes status updates to Uptime Kuma.

## Key Features

1. **Continuous HTTP/3 Monitoring** - Periodic health checks with configurable
   intervals
2. **Uptime Kuma Integration** - Push status updates via `/api/push/<pushToken>`
   endpoint
3. **Multi-Endpoint Support** - Monitor multiple HTTP/3 targets concurrently
4. **Command-Line Configuration** - All settings via flags (no hardcoded values)
5. **Detailed Error Reporting** - Include error messages in push notifications
6. **Backward Compatibility** - Preserve original fingerprint mode via
   `--fingerprint-only`

## Proposal Structure

```
openspec/changes/add-uptime-kuma-http3-monitor/
├── proposal.md          # Overview, scope, success criteria, risks
├── design.md            # Architecture, components, data flows, technical decisions
├── tasks.md             # Detailed implementation checklist (9 phases, ~40 tasks)
├── specs/               # Capability specifications with requirements
│   ├── http3-monitoring/spec.md
│   ├── push-integration/spec.md
│   └── command-line-interface/spec.md
└── README.md            # This summary file
```

## User Requirements (From Clarification Questions)

Based on user input during proposal creation:

- **Configuration Method**: Command-line parameters (flags)
- **Operating Mode**: Scheduled/periodic monitoring loop (not one-shot)
- **Failure Handling**: Include detailed error information in status pushes
- **Monitoring Scope**: Support multiple HTTP/3 endpoints

## Capability Specifications

### 1. HTTP/3 Monitoring ([specs/http3-monitoring/spec.md](specs/http3-monitoring/spec.md))

**Requirements:**

- Periodic HTTP/3 health checks with configurable intervals
- Multi-endpoint concurrent monitoring
- Configurable monitoring parameters (interval, timeout)
- Graceful shutdown on termination signals

**Example Scenario:**

```gherkin
Given a monitoring service is configured with an HTTP/3 endpoint target
And the monitoring interval is set to 60 seconds
When the monitoring timer fires for that endpoint
Then the service establishes a QUIC connection to the target
And the service reports the endpoint as healthy with response time
```

### 2. Push Integration ([specs/push-integration/spec.md](specs/push-integration/spec.md))

**Requirements:**

- Push API integration with Uptime Kuma `/api/push/<pushToken>` endpoint
- Per-endpoint push token configuration
- Correct parameter construction (status, msg, ping)
- Push request timeout (5s separate from check timeout)
- Comprehensive error logging and retry logic

**Example Scenario:**

```gherkin
Given an HTTP/3 health check completes successfully
And the response time is 245ms
When the monitoring service pushes the status to Uptime Kuma
Then the service constructs: /api/push/token?status=up&ping=245&msg=OK
And the service receives 200 OK response
And the service logs the successful push
```

### 3. Command-Line Interface ([specs/command-line-interface/spec.md](specs/command-line-interface/spec.md))

**Requirements:**

- Command-line flag parsing (no hardcoded values)
- Optional configuration flags with sensible defaults
- Backward compatibility mode (`--fingerprint-only`)
- Help and usage documentation
- URL validation for targets and Uptime Kuma endpoint

**Example Usage:**

```bash
# Single endpoint monitoring
./h3_monitor \
  --target https://endpoint1.com:443 \
  --sni endpoint1.com \
  --push-token token1 \
  --kuma-url https://uptime.example.com \
  --interval 60 \
  --timeout 10

# Multiple endpoints
./h3_monitor \
  --target https://ep1.com:443 --sni ep1.com --push-token token1 \
  --target https://ep2.com:443 --sni ep2.com --push-token token2 \
  --target https://ep3.com:443 --sni ep3.com --push-token token3

# Fingerprint-only mode (backward compatible)
./h3_monitor --fingerprint-only --target https://example.com:443 --sni example.com
```

## Implementation Plan

**Phases**: 9 (Foundation → HTTP/3 Client → Push Client → Monitoring Controller
→ Backward Compatibility → Logging → Documentation → Testing → Build)

**Estimated Time**: 9-13 hours (single developer) or 4-6 hours (with
parallelization)

**Key Tasks**:

1. CLI flag parsing and validation
2. Extract HTTP/3 check logic into reusable function
3. Implement Uptime Kuma push client with retry logic
4. Build concurrent monitoring service with graceful shutdown
5. Add fingerprint-only mode for backward compatibility
6. Implement structured logging
7. Write comprehensive documentation
8. Create unit and integration tests
9. Prepare release artifacts

See [tasks.md](tasks.md) for complete task breakdown with dependencies.

## Architecture Highlights

From [design.md](design.md):

**Component Structure:**

```
CLI Entry Point → Monitor Service Controller → Endpoint Monitors (goroutines)
                                                     ↓
                                    ┌────────────────┴────────────────┐
                                    ↓                                 ↓
                            HTTP/3 Client                    Uptime Kuma Push Client
                            - QUIC Transport                  - HTTP Request
                            - TLS Config                      - Status Report
                            - Fingerprint                     - Error Handling
```

**Technical Decisions:**

- Concurrent monitoring (one goroutine per endpoint)
- New HTTP/3 connection per check (realistic simulation)
- Blocking push with separate timeout (5s)
- Flag-based configuration (user requirement)
- Separate timeouts: 10s for HTTP/3, 5s for push

## Risk Mitigations

| Risk                                 | Mitigation                                    |
| ------------------------------------ | --------------------------------------------- |
| Uptime Kuma unavailable              | Buffer last status, retry with backoff        |
| Certificate changes                  | Log fingerprint changes, continue monitoring  |
| Memory leaks in long-running process | Connection pooling, periodic testing          |
| Too frequent polling                 | Default 60s interval, document best practices |

## Success Criteria

1. ✓ Service runs continuously and performs HTTP/3 checks at configured
   intervals
2. ✓ Successful connections push `status=up` with response time to Uptime Kuma
3. ✓ Failed connections push `status=down` with error message
4. ✓ Multiple endpoints monitored independently with individual tokens
5. ✓ Configuration entirely via command-line arguments
6. ✓ Process handles network errors gracefully and continues monitoring
7. ✓ Service responds to SIGINT/SIGTERM for graceful shutdown
8. ✓ Backward compatibility with `--fingerprint-only` mode

## Next Steps

1. **Review**: Read [proposal.md](proposal.md) for overview and scope
2. **Design Review**: Review [design.md](design.md) for technical decisions
3. **Tasks Review**: Review [tasks.md](tasks.md) for implementation plan
4. **Spec Review**: Review all three [specs/](specs/) for requirements
5. **Approval**: Approve proposal to begin implementation
6. **Implementation**: Execute tasks sequentially from [tasks.md](tasks.md)
7. **Validation**: Run
   `openspec validate add-uptime-kuma-http3-monitor --strict` after changes
8. **Archive**: After deployment, run
   `openspec archive add-uptime-kuma-http3-monitor`

## Validation Command

```bash
# Validate the proposal
openspec validate add-uptime-kuma-http3-monitor --strict

# View proposal details
openspec show add-uptime-kuma-http3-monitor

# List all active changes
openspec list

# List all specifications
openspec list --specs
```

## File References

- [proposal.md](proposal.md) - Why, what, impact
- [design.md](design.md) - Technical decisions and architecture
- [tasks.md](tasks.md) - Implementation checklist
- [specs/http3-monitoring/spec.md](specs/http3-monitoring/spec.md) - HTTP/3
  health check requirements
- [specs/push-integration/spec.md](specs/push-integration/spec.md) - Uptime Kuma
  push API requirements
- [specs/command-line-interface/spec.md](specs/command-line-interface/spec.md) -
  CLI configuration requirements

## Questions?

Refer to:

- [openspec/AGENTS.md](../../AGENTS.md) - OpenSpec usage instructions
- [openspec/project.md](../../project.md) - Project conventions
- Original issue: Transform `h3_fingerprint.go` into Uptime Kuma HTTP/3
  monitoring service
