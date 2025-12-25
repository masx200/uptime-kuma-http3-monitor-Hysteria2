# Capability: Command-Line Interface

## Overview

This capability defines the requirements for the command-line interface that allows users to configure the HTTP/3 monitoring service without hardcoded values.

## ADDED Requirements

### Requirement: Command-Line Flag Parsing

The monitoring service MUST accept all configuration via command-line arguments, with no hardcoded endpoint or token values.

#### Scenario: Required target flag

**Given** a user starts the monitoring service
**When** no `--target` flag is provided
**Then** the service exits with a non-zero status
**And** the service prints an error message indicating `--target` is required
**And** the service shows usage instructions

#### Scenario: Single target configuration

**Given** a user starts the service with: `--target https://example.com:443 --sni example.com --push-token abc123`
**When** the service parses the flags
**Then** the service configures one HTTP/3 endpoint
**And** the target URL is set to `https://example.com:443`
**And** the SNI server name is set to `example.com`
**And** the push token is set to `abc123`

#### Scenario: Multiple target configuration

**Given** a user starts the service with:
```
--target https://endpoint1.com:443 --sni endpoint1.com --push-token token1
--target https://endpoint2.com:443 --sni endpoint2.com --push-token token2
```
**When** the service parses the flags
**Then** the service configures two HTTP/3 endpoints
**And** endpoint 1 uses target1, sni1, and token1
**And** endpoint 2 uses target2, sni2, and token2
**And** each endpoint is monitored independently

#### Scenario: Flag pairing with mismatched counts

**Given** a user specifies 3 `--target` flags
**And** the user specifies only 2 `--push-token` flags
**When** the service parses the flags
**Then** target 1 is paired with token 1
**And** target 2 is paired with token 2
**And** target 3 is paired with token 2 (last token reused)
**And** the service logs a warning that token 2 is reused for target 3

### Requirement: Optional Configuration Flags

The monitoring service MUST provide optional flags for configuring service behavior with sensible defaults.

#### Scenario: Default monitoring interval

**Given** a user starts the service without `--interval` flag
**When** the service initializes
**Then** the monitoring interval defaults to 60 seconds
**And** the service logs: "Starting monitoring with interval: 60s"

#### Scenario: Custom monitoring interval

**Given** a user starts the service with `--interval 120`
**When** the service initializes
**Then** the monitoring interval is set to 120 seconds
**And** health checks occur every 120 seconds

#### Scenario: Default connection timeout

**Given** a user starts the service without `--timeout` flag
**When** the service initializes
**Then** the HTTP/3 connection timeout defaults to 10 seconds

#### Scenario: Custom connection timeout

**Given** a user starts the service with `--timeout 15`
**When** an HTTP/3 health check is performed
**Then** the connection timeout is set to 15 seconds

#### Scenario: Default Uptime Kuma URL

**Given** a user starts the service without `--kuma-url` flag
**When** the service initializes
**Then** the Uptime Kuma base URL defaults to `http://localhost:3001`

#### Scenario: Custom Uptime Kuma URL

**Given** a user starts the service with `--kuma-url https://monitoring.example.com`
**When** the service pushes status to Uptime Kuma
**Then** push requests are sent to `https://monitoring.example.com/api/push/<token>`

### Requirement: Backward Compatibility Mode

The monitoring service MUST preserve the original fingerprint extraction functionality for backward compatibility.

#### Scenario: Fingerprint-only mode

**Given** a user starts the service with `--fingerprint-only` flag
**And** provides `--target https://example.com:443 --sni example.com`
**When** the service runs
**Then** the service performs a single HTTP/3 connection
**And** the service prints the certificate SHA256 fingerprint
**And** the service exits immediately
**And** the output format matches the original `h3_fingerprint.go` tool exactly
**And** no monitoring loop is started
**And** no push requests are sent

#### Scenario: Fingerprint-only with multiple targets

**Given** a user starts the service with `--fingerprint-only` and multiple `--target` flags
**When** the service parses the flags
**Then** the service exits with an error
**And** the error message indicates fingerprint mode only supports a single target

#### Scenario: Implicit fingerprint mode (no push tokens)

**Given** a user starts the service with `--target` but no `--push-token`
**And** the `--fingerprint-only` flag is NOT specified
**When** the service runs
**Then** the service logs a deprecation warning
**And** the warning suggests using `--fingerprint-only` explicitly
**And** the service runs in fingerprint-only mode
**And** the service exits after printing the fingerprint

### Requirement: Help and Usage Documentation

The monitoring service MUST provide clear usage documentation via command-line flags.

#### Scenario: Help flag displays usage

**Given** a user runs the service with `--help` flag
**When** the help text is displayed
**Then** the service shows all available flags
**And** each flag includes a description
**And** required flags are marked as required
**And** default values are shown for optional flags
**And** example usage is provided
**And** the service exits after displaying help

#### Scenario: Invalid flag

**Given** a user runs the service with an unrecognized flag `--invalid-flag`
**When** the service parses the flags
**Then** the service exits with a non-zero status
**And** the service prints an error about the invalid flag
**And** the service suggests using `--help` to see available flags

### Requirement: URL Validation

The monitoring service MUST validate URL formats and schemes provided via command-line flags.

#### Scenario: Valid HTTPS URL for target

**Given** a user provides `--target https://example.com:443`
**When** the service validates the URL
**Then** the URL is accepted
**And** the scheme is confirmed as HTTPS
**And** the hostname and port are parsed correctly

#### Scenario: Invalid URL scheme for target

**Given** a user provides `--target http://example.com:80`
**When** the service validates the URL
**Then** the service exits with an error
**And** the error message indicates only HTTPS URLs are supported for HTTP/3

#### Scenario: Malformed URL

**Given** a user provides `--target not-a-valid-url`
**When** the service validates the URL
**Then** the service exits with an error
**And** the error message indicates the URL is malformed

#### Scenario: Valid Uptime Kuma URL

**Given** a user provides `--kuma-url http://localhost:3001`
**When** the service validates the URL
**Then** the URL is accepted
**And** the URL can use HTTP or HTTPS scheme

### Requirement: Flag Documentation Standards

All command-line flags MUST follow consistent documentation standards.

#### Scenario: Flag naming conventions

**Given** the service defines command-line flags
**When** examining flag names
**Then** all flags use kebab-case (e.g., `--push-token`, not `--pushToken`)
**And** flag names are descriptive and clear
**And** boolean flags use positive wording (e.g., `--fingerprint-only`)

#### Scenario: Flag description format

**Given** the help text is displayed
**When** reading flag descriptions
**Then** each description explains the flag's purpose
**And** descriptions indicate required vs optional status
**And** descriptions show default values where applicable
**And** descriptions provide examples for complex flags

## MODIFIED Requirements

### Requirement: Main Function Behavior

The main function MUST support both the original fingerprint extraction behavior and the new monitoring behavior based on command-line flags.

#### Scenario: Detect execution mode

**Given** the service starts
**When** parsing command-line flags
**Then** if `--fingerprint-only` is present, run in fingerprint mode
**And** if `--push-token` is present, run in monitoring mode
**And** if neither is present, run in fingerprint mode with deprecation warning

## Cross-References

- **HTTP/3 Monitoring Capability**: Target URLs and SNI names from CLI are used for health checks
- **Push Integration Capability**: Push tokens and Kuma URL from CLI are used for status reporting
- **Configuration Capability**: All service parameters are configurable via CLI as defined in this capability
