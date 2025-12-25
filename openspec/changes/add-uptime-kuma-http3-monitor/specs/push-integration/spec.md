# Capability: Uptime Kuma Push Integration

## Overview

This capability defines the requirements for integrating HTTP/3 health check
results with Uptime Kuma's push monitoring API, enabling external services to
report status programmatically.

## ADDED Requirements

### Requirement: Push API Integration

The monitoring service MUST send HTTP/3 health check results to Uptime Kuma via
the `/api/push/<pushToken>` endpoint.

#### Scenario: Successful push after successful health check

**Given** an HTTP/3 health check completes successfully **And** the response
time is 245ms **And** the endpoint has a configured push token "abc123" **When**
the monitoring service pushes the status to Uptime Kuma **Then** the service
constructs the URL:
`http://kuma-host:3001/api/push/abc123?status=up&ping=245&msg=OK` **And** the
service sends an HTTP GET request to the push endpoint **And** the service
receives a 200 OK response with `{"ok": true}` **And** the service logs the
successful push

#### Scenario: Push after failed health check

**Given** an HTTP/3 health check fails with error "dial timeout: no connection
established" **And** the endpoint has a configured push token "xyz789" **When**
the monitoring service pushes the status to Uptime Kuma **Then** the service
constructs the URL with:
`status=down&msg=dial+timeout%3A+no+connection+established` **And** the service
URL-encodes the error message for safe transmission **And** the service receives
a 200 OK response **And** the service logs the push with "down" status

#### Scenario: Push endpoint returns 404 Not Found

**Given** an HTTP/3 health check completes (success or failure) **And** the
configured push token is invalid or the monitor is disabled **When** the
monitoring service pushes the status to Uptime Kuma **Then** the service
receives a 404 Not Found response with
`{"ok": false, "msg": "Monitor not found or not active"}` **And** the service
logs a critical error indicating token validation issue **And** the service
continues monitoring (does not exit) **And** the service does not retry the push
for this check

#### Scenario: Push endpoint returns 5xx server error

**Given** an HTTP/3 health check completes **And** the Uptime Kuma server is
experiencing internal errors **When** the monitoring service pushes the status
to Uptime Kuma **Then** the service receives a 500 Internal Server Error
response **And** the service logs a warning about the server error **And** the
service waits 1 second **And** the service retries the push request once **And**
if the retry succeeds, the service logs success **And** if the retry fails, the
service logs the error and continues monitoring

#### Scenario: Push endpoint network error

**Given** an HTTP/3 health check completes **And** the monitoring service cannot
reach the Uptime Kuma push endpoint **When** the monitoring service attempts to
push the status **Then** the service logs a warning about the network error
**And** the service does not retry the push **And** the service continues with
the next monitoring cycle

### Requirement: Push Request Configuration

The monitoring service MUST support configuration of Uptime Kuma endpoint and
push tokens per monitored HTTP/3 endpoint.

#### Scenario: Default Uptime Kuma URL

**Given** a monitoring service is started without specifying a Uptime Kuma URL
**When** the service initializes **Then** the Uptime Kuma base URL defaults to
`http://localhost:3001` **And** all push requests are sent to this default
endpoint

#### Scenario: Custom Uptime Kuma URL

**Given** a monitoring service is started with
`--kuma-url https://uptime.example.com` **When** the service pushes status to
Uptime Kuma **Then** push requests are sent to
`https://uptime.example.com/api/push/<token>`

#### Scenario: Endpoint-specific push tokens

**Given** a monitoring service is configured with 3 HTTP/3 endpoints **And** the
first endpoint has push token "token1" **And** the second endpoint has push
token "token2" **And** the third endpoint has push token "token3" **When**
health checks complete for all endpoints **Then** each endpoint's status is
pushed to its respective push token **And** token1 receives status for endpoint
1 only **And** token2 receives status for endpoint 2 only **And** token3
receives status for endpoint 3 only

#### Scenario: Token reuse when fewer tokens than endpoints

**Given** a monitoring service is configured with 3 HTTP/3 endpoints **And**
only one push token "shared-token" is provided **When** health checks complete
for all endpoints **Then** all three endpoints push status to "shared-token"
**And** the service logs that the token is being reused for multiple endpoints

### Requirement: Push Request Parameters

The monitoring service MUST construct push requests with correct parameters as
defined by Uptime Kuma's API specification.

#### Scenario: Status parameter mapping

**Given** an HTTP/3 health check result **When** constructing the push request
**Then** a successful check maps to `status=up` **And** a failed check maps to
`status=down` **And** the status parameter is always included in the query
string

#### Scenario: Message parameter construction

**Given** an HTTP/3 health check succeeds **When** constructing the push request
**Then** the message parameter defaults to `msg=OK` **And** the OK message is
URL-safe

**Given** an HTTP/3 health check fails with error "context deadline exceeded"
**When** constructing the push request **Then** the message parameter includes
the error: `msg=context+deadline+exceeded` **And** the error message is
URL-encoded **And** if the error message exceeds 250 characters, it is truncated

#### Scenario: Ping parameter for response time

**Given** an HTTP/3 health check completes with response time 245.5ms **When**
constructing the push request **Then** the ping parameter is set to the integer
value: `ping=245` **And** fractional milliseconds are rounded to the nearest
integer

**Given** an HTTP/3 health check fails **When** constructing the push request
**Then** the ping parameter is omitted from the query string

#### Scenario: Multiple parameters combined

**Given** an HTTP/3 health check succeeds with 245ms response time **And** the
push token is "mytoken" **When** constructing the full push URL **Then** the URL
format is: `http://kuma:3001/api/push/mytoken?status=up&ping=245&msg=OK` **And**
all parameters are properly ordered and URL-encoded

### Requirement: Push Request Timeout

The monitoring service MUST enforce a separate timeout for push requests to
prevent monitoring delays.

#### Scenario: Successful push within timeout

**Given** the push request timeout is set to 5 seconds **When** the monitoring
service sends a push request **And** the Uptime Kuma server responds within 5
seconds **Then** the request completes successfully **And** the response is
processed normally

#### Scenario: Push request timeout

**Given** the push request timeout is set to 5 seconds **When** the monitoring
service sends a push request **And** the Uptime Kuma server does not respond
within 5 seconds **Then** the request is canceled **And** the service logs a
warning about push timeout **And** the service continues with the next
monitoring cycle **And** the service does not retry a timed-out push request

### Requirement: Push Error Logging

The monitoring service MUST log push-related errors with appropriate severity
and context for troubleshooting.

#### Scenario: Successful push logging

**Given** a push request succeeds **When** the response is received **Then** the
service logs an INFO-level message **And** the log includes endpoint name,
status, and response time **And** the log format is:
`endpoint="Production" status="up" ping=245ms push=OK`

#### Scenario: Push failure logging

**Given** a push request fails with a 404 response **When** the error is
detected **Then** the service logs a WARN-level message **And** the log includes
the endpoint name and the error response **And** the log format includes:
`push failed: 404 Not Found - Check push token and monitor activation`

#### Scenario: Critical push error logging

**Given** a push request fails with 404 on the first check **When** the error is
detected **Then** the service logs an ERROR-level message **And** the log
suggests verifying the push token in Uptime Kuma configuration **And** the
service includes the push token prefix (e.g., "abc...xyz") for identification

## MODIFIED Requirements

None - this is a new capability with no modifications to existing requirements.

## Cross-References

- **HTTP/3 Monitoring Capability**: Health check results from this capability
  are consumed by push integration
- **Configuration Capability**: Push tokens and Uptime Kuma URL are configured
  via command-line flags
