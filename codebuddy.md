<!-- OPENSPEC:START -->

# OpenSpec Instructions

These instructions are for AI assistants working in this project.

Always open `@/openspec/AGENTS.md` when the request:

- Mentions planning or proposals (words like proposal, spec, change, plan)
- Introduces new capabilities, breaking changes, architecture shifts, or big
  performance/security work
- Sounds ambiguous and you need the authoritative spec before coding

Use `@/openspec/AGENTS.md` to learn:

- How to create and apply change proposals
- Spec format and conventions
- Project structure and guidelines

Keep this managed block so 'openspec update' can refresh the instructions.

<!-- OPENSPEC:END -->

# Project Overview

This project has two components:

1. **`h3_monitor` (Go)** — Primary. Continuous HTTP/3 endpoint monitor that pushes health check results to Uptime Kuma via its Push API. Single-file implementation in `h3_fingerprint.go`.
2. **Node.js/sing-box proxy service** — Secondary. Network proxy using sing-box with TUIC, Hysteria2, and Reality protocols, for low-memory (128MB+) VPS environments. Shell-based (`index.js`, `warp.sh`, `start.sh`).

## Build & Run Commands

```bash
# Build the Go monitor
go build -o h3_monitor h3_fingerprint.go

# Run in fingerprint-only mode (single check, then exit)
./h3_monitor --target https://example.com:443 --fingerprint-only

# Run in monitoring mode (continuous, pushes to Uptime Kuma)
./h3_monitor --target https://example.com:443 --push-token=xxxxx --kuma-url=https://kuma.example.com

# Multi-endpoint (repeat --target and --push-token flags)
./h3_monitor --target https://a.com:443 --push-token=aaa --target https://b.com:443 --push-token=bbb

# Run without building
go run h3_fingerprint.go [flags]

# Cross-compile
GOOS=linux GOARCH=arm64 go build -o h3_monitor h3_fingerprint.go

# Run the Node.js proxy service (Linux only)
npm start

# No tests exist yet — go test ./... finds no test files
# testify and gomock are in go.sum but not yet used
```

## Go Monitor Architecture

All Go code lives in `h3_fingerprint.go` (single file, ~770 lines):

```
main() → parseFlags() → mode router
  │
  ├─ --fingerprint-only → runFingerprintOnly() → exit
  │
  └─ monitoring mode → startMonitoring()
       │
       ├─ goroutine: monitorEndpoint(ep1) → ticker → checkAndPush()
       ├─ goroutine: monitorEndpoint(ep2) → ticker → checkAndPush()
       └─ goroutine: monitorEndpoint(epN) → ticker → checkAndPush()
                                                │
                                                ├─ CheckHTTP3() — http3.Transport, 3 retries, cert fingerprint
                                                └─ PushStatus() — HTTP GET /api/push/{token}?status=up|down&ping=N
```

Key data structures: `EndpointConfig`, `Config`, `CheckResult`, `KumaPushResponse`

### Key Design Decisions

- **Single-file monolith** — no package splitting
- **No config files** — all configuration via CLI flags only
- **New HTTP/3 connection per check** — no connection pooling (intentional, simulates real client)
- **InsecureSkipVerify: true** — TLS verification disabled (cert fingerprint is validated instead)
- **Per-endpoint goroutines** — failures in one endpoint don't block others
- **Token reuse** — if fewer `--push-token` values than `--target` values, the last token is reused
- **Graceful shutdown** — SIGINT triggers `close(stopCh)`, `wg.Wait()` with 30s timeout

### Retry Logic

- `CheckHTTP3()`: 3 retries with 500ms sleep between attempts
- `PushStatus()`: called from `checkAndPush()` which retries up to 3 times on 5xx errors with 1s delay

### CLI Flags

`--target`, `--sni`, `--host`, `--method`, `--push-token`, `--fingerprint`, `--expected-status`, `--kuma-url`, `--interval`, `--timeout`, `--fingerprint-only`. Target URLs must use `https://` scheme.

## Key Dependency

`github.com/quic-go/quic-go v0.58.0` — HTTP/3 (QUIC) transport. This is the only direct dependency.

## Node.js Proxy Service

Orchestration: `index.js` spawns `warp.sh` (Cloudflare WARP tunnel on SOCKS5 :1080) then `start.sh` (downloads sing-box, generates config, starts proxy with TUIC/Hysteria2/Reality inbounds, generates subscription URLs). Automatic daily restart at 00:03 Beijing time. Linux-only, requires Bash.
