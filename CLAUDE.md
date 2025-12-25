<!-- OPENSPEC:START -->
# OpenSpec Instructions

These instructions are for AI assistants working in this project.

Always open `@/openspec/AGENTS.md` when the request:
- Mentions planning or proposals (words like proposal, spec, change, plan)
- Introduces new capabilities, breaking changes, architecture shifts, or big performance/security work
- Sounds ambiguous and you need the authoritative spec before coding

Use `@/openspec/AGENTS.md` to learn:
- How to create and apply change proposals
- Spec format and conventions
- Project structure and guidelines

Keep this managed block so 'openspec update' can refresh the instructions.

<!-- OPENSPEC:END -->

# CLAUDE.md

This file contains additional context for the Claude Code agent to help it be
more productive when working in this codebase.

## Overview

This is a Node.js project that implements a network proxy service using sing-box
with support for multiple proxy protocols (TUIC, Hysteria2, and Reality). The
project is designed for low-memory environments (128MB+ RAM) and includes
automatic daily restart functionality for cache clearing.

## Architecture and Structure

### Core Components

1. **index.js** - Main Node.js entry point that orchestrates the execution of
   bash scripts
   - Executes `warp.sh` to download and run masque-plus proxy tools
   - Executes `start.sh` to configure and run sing-box with multiple protocols

2. **warp.sh** - Downloads and runs proxy tools
   - Downloads `masque-plus` and `usque` binaries from CDN
   - Runs masque-plus in an infinite loop with specific configuration
   - Connects to Cloudflare WARP endpoints (162.159.198.2:443)

3. **start.sh** - Main configuration and service script
   - Downloads sing-box binary based on system architecture
   - Generates and persists UUID and Reality keypair
   - Creates SSL certificates (self-signed or OpenSSL-generated)
   - Configures and starts sing-box with multiple protocols
   - Implements daily restart at 00:03 Beijing time
   - Generates subscription URLs for clients

4. **h3_fingerprint.go** - Go utility for HTTP/3 certificate fingerprint
   extraction
   - Connects to HTTP/3 endpoints and extracts TLS certificate SHA256
     fingerprint
   - Used for certificate validation in Hysteria2 protocol configuration

### Protocol Configuration

The service supports three proxy protocols:

- **TUIC** - QUIC-based proxy protocol with congestion control (BBR)
- **Hysteria2** - High-speed UDP-based proxy with masquerading
- **Reality** - VLESS protocol with TLS obfuscation

All protocols share the same UUID for authentication and use custom TLS
certificates.

## Development Environment

### Prerequisites

- Node.js 18+ (required)
- Bash/shell environment (Linux/Unix)
- wget or curl for downloads
- OpenSSL (optional, for better certificate generation)

### Project Structure

```
/
├── index.js          # Main Node.js entry point
├── package.json      # Node.js project configuration
├── warp.sh           # Proxy tool download and execution
├── start.sh          # Main service configuration script
├── h3_fingerprint.go # Certificate fingerprint utility
├── go.mod           # Go module dependencies
├── .gitignore       # Git ignore rules
├── README.md        # User documentation
└── .npm/            # Runtime directory (created at runtime)
    ├── uuid.txt     # Persistent UUID storage
    ├── key.txt      # Reality keypair storage
    ├── config.json  # sing-box configuration
    ├── list.txt     # Subscription URLs
    └── sub.txt      # Base64-encoded subscription
```

### Environment Variables

- `TUIC_PORT` - Port for TUIC protocol (optional, defaults to empty)
- `HY2_PORT` - Port for Hysteria2 protocol (optional, defaults to empty)
- `REALITY_PORT` - Port for Reality protocol (optional, defaults to empty)

## Common Development Commands

### Running the Application

```bash
# Install dependencies (minimal for this project)
npm install

# Start the service
npm start
# or
node index.js
```

### Development Tasks

```bash
# Build the Go fingerprint utility
go build -o h3_fingerprint.exe h3_fingerprint.go

# Run fingerprint utility to get certificate SHA256
./h3_fingerprint.exe
```

### Testing Protocol Connectivity

```bash
# Test HTTP/3 connectivity with custom fingerprint
go run h3_fingerprint.go

# Test sing-box configuration after generation
.npm/sing-box check -c .npm/config.json
```

## Key Technical Details

### Port Configuration

- Ports can be shared between protocols (TCP/UDP)
- Default environment variables in index.js: REALITY_PORT=20143, HY2_PORT=20143
- TUIC requires empty port to disable if not needed

### Certificate Management

- Self-signed certificates hardcoded for systems without OpenSSL
- OpenSSL-generated certificates use "www.bing.com" as CN
- Certificates stored with 600 permissions in .npm/ directory
- SHA256 fingerprint extraction for Hysteria2 pinning

### UUID and Key Persistence

- UUID generated once and persisted in .npm/uuid.txt
- Reality keypair generated once and persisted in .npm/key.txt
- Both files have 600 permissions for security

### Architecture Detection and Downloads

- Automatic detection of ARM64, AMD64, and S390x architectures
- Downloads sing-box from architecture-specific URLs
- Files downloaded with random names for security

### Daily Restart Mechanism

- Automatic restart at 00:03 Beijing time (UTC+8)
- Kills and restarts sing-box process
- Designed to clear cache and maintain stability

### Subscription Generation

- Generates client configuration URLs for all enabled protocols
- Base64-encoded subscription saved to .npm/sub.txt
- Includes ISP information and protocol identifiers

## Security Considerations

1. **Hardcoded Credentials** - warp.sh contains hardcoded WARP credentials
2. **Permission Management** - Sensitive files use 600 permissions
3. **TLS Configuration** - Self-signed certificates with InsecureSkipVerify
4. **Process Management** - Proper PID tracking for graceful restarts

## Deployment Notes

- Designed for low-memory environments (128MB+ RAM)
- Not recommended for 64MB environments (like freecloudpanel)
- Requires outbound internet access for downloads
- Persistent storage required for .npm directory
- Bash environment required for script execution

## Troubleshooting

### Common Issues

1. **Download Failures** - Check internet connectivity and CDN availability
2. **Permission Errors** - Ensure script has execute permissions
3. **Port Conflicts** - Verify ports are not in use by other services
4. **Memory Issues** - Monitor RAM usage on low-memory systems

### Debug Commands

```bash
# Check sing-box process
ps aux | grep sing-box

# View generated configuration
cat .npm/config.json

# Check subscription URLs
cat .npm/list.txt

# Test individual protocols
curl -v --http3 https://[IP]:[PORT]
```
