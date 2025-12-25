# Capability: HTTP/3 Monitoring

## Overview

This capability defines the requirements for continuous HTTP/3 endpoint health
monitoring, including periodic connectivity checks, response time measurement,
and certificate validation.

## ADDED Requirements

### Requirement: Periodic HTTP/3 Health Checks

The monitoring service MUST continuously check the health of configured HTTP/3
endpoints at a configurable interval.

#### Scenario: Successful HTTP/3 connection check

**Given** a monitoring service is configured with an HTTP/3 endpoint target
**And** the monitoring interval is set to 60 seconds **When** the monitoring
timer fires for that endpoint **Then** the service establishes a QUIC connection
to the target **And** the service completes a TLS handshake with the configured
SNI server name **And** the service executes an HTTP HEAD request **And** the
service measures the response time **And** the service reports the endpoint as
healthy

#### Scenario: HTTP/3 connection timeout

**Given** a monitoring service is configured with an HTTP/3 endpoint target
**And** the connection timeout is set to 10 seconds **When** the monitoring
timer fires for that endpoint **And** the target endpoint does not respond
within the timeout period **Then** the service cancels the connection attempt
**And** the service reports the endpoint as unhealthy **And** the service
includes "timeout" in the error message

#### Scenario: HTTP/3 certificate validation

**Given** a monitoring service is configured with an HTTP/3 endpoint target
**And** InsecureSkipVerify is enabled **When** the monitoring timer fires for
that endpoint **And** the TLS handshake completes successfully **Then** the
service extracts the server's TLS certificate **And** the service calculates the
SHA256 fingerprint of the certificate **And** the service logs the certificate
fingerprint for reference

#### Scenario: Response time measurement

**Given** a monitoring service is checking an HTTP/3 endpoint **When** the HTTP
HEAD request is initiated **And** the response is received **Then** the service
measures the elapsed time in milliseconds **And** the service reports the
response time with millisecond precision **And** the response time includes QUIC
handshake and HTTP transaction duration

### Requirement: Multi-Endpoint Concurrent Monitoring

The monitoring service MUST support monitoring multiple HTTP/3 endpoints
simultaneously, with independent health checks for each endpoint.

#### Scenario: Multiple endpoints configured

**Given** a monitoring service is configured with 3 HTTP/3 endpoint targets
**And** each endpoint has a unique target URL and SNI configuration **When** the
monitoring service starts **Then** the service launches a separate goroutine for
each endpoint **And** each goroutine performs health checks independently
**And** a failure in one endpoint does not affect monitoring of other endpoints

#### Scenario: Endpoint-specific configuration

**Given** a monitoring service is configured with multiple endpoints **When**
parsing endpoint configurations **Then** each endpoint can have a unique target
URL **And** each endpoint can have a unique SNI server name **And** all
endpoints share the same monitoring interval and timeout **And** endpoint
failures are reported with the endpoint name for identification

### Requirement: Configurable Monitoring Parameters

The monitoring service MUST allow configuration of monitoring intervals and
timeouts via command-line arguments.

#### Scenario: Default monitoring interval

**Given** a monitoring service is started without specifying an interval
**When** the service initializes **Then** the monitoring interval defaults to 60
seconds **And** the service logs the configured interval at startup

#### Scenario: Custom monitoring interval

**Given** a monitoring service is started with `--interval 120` **When** the
service initializes **Then** the monitoring interval is set to 120 seconds
**And** health checks occur every 120 seconds for each endpoint

#### Scenario: Connection timeout configuration

**Given** a monitoring service is started with `--timeout 15` **When** an HTTP/3
health check is performed **Then** the connection timeout is set to 15 seconds
**And** the check is canceled if no response within 15 seconds

#### Scenario: Minimum and maximum interval validation

**Given** a monitoring service is started with `--interval 5` **When** the
service initializes **Then** the service accepts the 5-second interval **And**
the service logs a warning that intervals below 10 seconds may overwhelm targets

### Requirement: Graceful Shutdown

The monitoring service MUST handle termination signals gracefully, allowing
in-progress health checks to complete before exiting.

#### Scenario: SIGINT signal handling

**Given** a monitoring service is running with active endpoint monitoring
**When** a SIGINT signal (Ctrl+C) is received **Then** the service stops
creating new health checks **And** the service waits for in-progress health
checks to complete (max 30 seconds) **And** the service shuts down cleanly after
all checks complete **And** the service logs a shutdown message

#### Scenario: SIGTERM signal handling

**Given** a monitoring service is running with active endpoint monitoring
**When** a SIGTERM signal is received **Then** the service behaves identically
to SIGINT handling **And** all in-progress checks are allowed to complete

#### Scenario: Forced shutdown after timeout

**Given** a monitoring service is shutting down **And** an in-progress health
check exceeds 30 seconds **When** the graceful shutdown timeout is reached
**Then** the service forcibly exits **And** the service logs that shutdown
timeout was exceeded

## MODIFIED Requirements

### Requirement: HTTP/3 Connection Logic

The existing HTTP/3 connection logic from `h3_fingerprint.go` MUST be refactored
into a reusable function that supports both one-time checks and continuous
monitoring.

#### Scenario: Backward compatibility for fingerprint extraction

**Given** a user runs the tool with `--fingerprint-only` flag **When** the
HTTP/3 check completes successfully **Then** the tool prints the certificate
SHA256 fingerprint **And** the tool exits immediately without starting
monitoring **And** the output format matches the original tool exactly

#### Scenario: Reusable check function

**Given** the monitoring service needs to perform an HTTP/3 health check
**When** the `CheckHTTP3()` function is called with target, SNI, and timeout
**Then** the function returns a CheckResult struct **And** the CheckResult
includes success status, response time, fingerprint, and error message **And**
the function can be called repeatedly in a monitoring loop

## Cross-References

- **Push Integration Capability**: Health check results are reported to Uptime
  Kuma via push endpoint
- **Configuration Capability**: Endpoints are configured via command-line flags
  as defined in configuration requirements
