# Uptime Kuma HTTP/3 Monitor

一个用于监控 HTTP/3 端点并通过 Uptime Kuma Push API 推送状态的工具。

[English](#english-version) | [中文](#中文版本)

---

## 中文版本

### 概述

`h3_monitor` 是一个持续监控 HTTP/3 端点健康状态的工具，它可以将检测结果实时推送到 Uptime Kuma 监控系统。该工具支持同时监控多个端点，并提供详细的错误报告和响应时间测量。

### 主要特性

- ✅ **HTTP/3 协议支持** - 使用 QUIC 协议进行连接测试
- ✅ **多端点监控** - 同时监控多个 HTTP/3 目标
- ✅ **Uptime Kuma 集成** - 通过 Push API 实时推送状态
- ✅ **命令行配置** - 灵活的参数配置，无需硬编码
- ✅ **详细错误报告** - 包含完整的错误信息用于调试
- ✅ **响应时间测量** - 精确测量毫秒级响应时间
- ✅ **证书指纹提取** - 可选提取 TLS 证书 SHA256 指纹
- ✅ **优雅关闭** - 支持 SIGINT/SIGTERM 信号处理

### 前置要求

- Go 1.25.4+
- Uptime Kuma 实例（用于接收状态推送）
- HTTP/3 端点（需要监控的目标服务）

### 安装

#### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/masx200/uptime-kuma-http3-monitor.git
cd uptime-kuma-http3-monitor

# 构建工具
go build -o h3_monitor h3_fingerprint.go

# 或者使用交叉编译
GOOS=linux GOARCH=amd64 go build -o h3_monitor-linux-amd64 h3_fingerprint.go
GOOS=windows GOARCH=amd64 go build -o h3_monitor.exe h3_fingerprint.go
GOOS=darwin GOARCH=amd64 go build -o h3_monitor-darwin-amd64 h3_fingerprint.go
```

### 使用方法

#### 1. 基本用法 - 监控单个端点

```bash
./h3_monitor \
  --target https://example.com:443 \
  --sni example.com \
  --push-token YOUR_PUSH_TOKEN \
  --kuma-url http://localhost:3001 \
  --interval 60
```

**参数说明：**
- `--target`: HTTP/3 端点的 URL（必需）
- `--sni`: TLS SNI 服务器名称（必需）
- `--push-token`: Uptime Kuma 推送令牌（必需）
- `--kuma-url`: Uptime Kuma 实例地址（默认：http://localhost:3001）
- `--interval`: 监控间隔，单位秒（默认：60）
- `--timeout`: 连接超时时间，单位秒（默认：10）

#### 2. 监控多个端点

```bash
./h3_monitor \
  --target https://endpoint1.com:443 --sni endpoint1.com --push-token TOKEN1 \
  --target https://endpoint2.com:443 --sni endpoint2.com --push-token TOKEN2 \
  --target https://endpoint3.com:443 --sni endpoint3.com --push-token TOKEN3 \
  --interval 60
```

**注意：** 如果推送令牌数量少于端点数量，最后一个令牌将被重用。

#### 3. 仅提取证书指纹（向后兼容）

```bash
./h3_monitor \
  --fingerprint-only \
  --target https://example.com:443 \
  --sni example.com
```

此模式运行一次后退出，输出证书的 SHA256 指纹，保持与原工具的兼容性。

### Uptime Kuma 配置

#### 创建 Push 监控

1. 在 Uptime Kuma 中添加新监控
2. 选择监控类型为 **Push** (推送)
3. 配置监控名称和描述
4. 复制生成的 **Push Token**
5. 在命令行中使用该 token：`--push-token YOUR_TOKEN`

#### Push API 格式

工具将向 Uptime Kuma 发送以下格式的请求：

**成功状态：**
```
GET /api/push/YOUR_TOKEN?status=up&ping=245&msg=OK
```

**失败状态：**
```
GET /api/push/YOUR_TOKEN?status=down&msg=dial+timeout%3A+no+connection+established
```

### 命令行参数

| 参数 | 类型 | 必需 | 默认值 | 描述 |
|------|------|------|--------|------|
| `--target` | URL | 是 | 无 | HTTP/3 端点 URL（可多次指定） |
| `--sni` | 字符串 | 是 | 无 | TLS SNI 服务器名称（可多次指定） |
| `--push-token` | 字符串 | 是* | 无 | Uptime Kuma 推送令牌（可多次指定） |
| `--kuma-url` | URL | 否 | http://localhost:3001 | Uptime Kuma 实例地址 |
| `--interval` | 整数 | 否 | 60 | 监控间隔（秒） |
| `--timeout` | 整数 | 否 | 10 | HTTP/3 连接超时（秒） |
| `--fingerprint-only` | 布尔 | 否 | false | 仅提取证书指纹并退出 |

*注：如果不提供 `--push-token`，工具将进入指纹提取模式（向后兼容）

### 日志输出

工具使用结构化日志格式，包含时间戳、日志级别和详细信息：

```
2025-12-25T10:00:00Z [INFO] Starting HTTP/3 monitoring service (interval=60s, timeout=10s)
2025-12-25T10:00:00Z [INFO] Spawning monitor for endpoint "endpoint1.com"
2025-12-25T10:00:01Z [INFO] endpoint="endpoint1.com" status="up" ping=245ms msg="OK"
2025-12-25T10:01:01Z [INFO] endpoint="endpoint1.com" status="up" ping=238ms msg="OK"
2025-12-25T10:02:01Z [ERROR] endpoint="endpoint1.com" status="down" msg="dial timeout: no connection established"
2025-12-25T10:02:01Z [WARN] endpoint="endpoint1.com" push failed: 503 Service Unavailable, retrying...
```

### Docker 部署

#### Dockerfile 示例

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY h3_fingerprint.go go.mod go.sum ./
RUN CGO_ENABLED=0 GOOS=linux go build -o h3_monitor h3_fingerprint.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/h3_monitor .
CMD ["./h3_monitor"]
```

#### docker-compose.yml 示例

```yaml
version: '3.8'
services:
  h3-monitor:
    build: .
    container_name: h3_monitor
    restart: unless-stopped
    command:
      - --target
      - https://example.com:443
      - --sni
      - example.com
      - --push-token
      - ${PUSH_TOKEN}
      - --kuma-url
      - http://uptime-kuma:3001
      - --interval
      - "60"
    environment:
      - PUSH_TOKEN=your_token_here
```

运行：
```bash
docker-compose up -d
```

### 系统服务配置 (systemd)

创建服务文件 `/etc/systemd/system/h3-monitor.service`：

```ini
[Unit]
Description=HTTP/3 Monitoring Service
After=network.target

[Service]
Type=simple
User=monitoring
WorkingDirectory=/opt/h3-monitor
ExecStart=/opt/h3-monitor/h3_monitor \
  --target https://example.com:443 \
  --sni example.com \
  --push-token YOUR_TOKEN \
  --kuma-url http://localhost:3001 \
  --interval 60

Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

启用和启动服务：
```bash
sudo systemctl daemon-reload
sudo systemctl enable h3-monitor
sudo systemctl start h3-monitor
sudo systemctl status h3-monitor
```

### 故障排查

#### 问题：工具显示 "404 Not Found"

**原因：** Push Token 无效或对应的监控已禁用

**解决方案：**
1. 检查 Uptime Kuma 中监控是否处于激活状态
2. 验证 Push Token 是否正确复制
3. 确认监控类型为 "Push"

#### 问题：连接超时

**原因：** 目标端点不可达或防火墙阻止

**解决方案：**
1. 检查网络连接：`curl -v --http3 https://target:port`
2. 验证防火墙规则允许 UDP 流量（QUIC 使用 UDP）
3. 增加 `--timeout` 值

#### 问题：频繁推送失败

**原因：** Uptime Kuma 实例不可用或网络问题

**解决方案：**
1. 检查 Uptime Kuma 服务状态
2. 验证 `--kuma-url` 配置正确
3. 查看日志中的具体错误信息

### 性能考虑

- **内存使用**: 基础 ~50MB + 每个端点 ~5MB
- **CPU 占用**: 空闲时极小，连接建立时有峰值
- **网络流量**: 每个端点每次检查 ~2-5KB
- **推荐配置**: 最多监控 10 个端点，间隔不少于 10 秒

### 安全建议

1. **保护 Push Token**
   - 不要在命令行中直接使用（进程列表可见）
   - 考虑使用环境变量（未来版本支持）
   - 限制 token 权限为只读

2. **网络安全**
   - 使用 HTTPS 连接 Uptime Kuma
   - 在生产环境中禁用 `InsecureSkipVerify`

3. **日志安全**
   - 日志中不包含完整的 Push Token
   - 仅显示 token 的前缀和后缀

### 开发

#### 项目结构

```
.
├── h3_fingerprint.go       # 主程序源码
├── go.mod                  # Go 模块依赖
├── go.sum                  # 依赖校验和
├── openspec/               # 规格和设计文档
│   └── changes/
│       └── add-uptime-kuma-http3-monitor/
│           ├── proposal.md     # 提案概述
│           ├── design.md       # 技术设计
│           ├── tasks.md        # 实现任务
│           └── specs/          # 能力规格
└── README.md               # 本文档
```

#### 运行测试

```bash
# 单元测试（待实现）
go test ./...

# 集成测试（待实现）
go test -tags=integration ./...
```

### 贡献

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

### 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

### 致谢

- [quic-go](https://github.com/quic-go/quic-go) - QUIC 协议的 Go 语言实现
- [Uptime Kuma](https://github.com/louislam/uptime-kuma) - 优秀的自托管监控工具

### 联系方式

- 提交问题: [GitHub Issues](https://github.com/masx200/uptime-kuma-http3-monitor/issues)
- 讨论: [GitHub Discussions](https://github.com/masx200/uptime-kuma-http3-monitor/discussions)

---

## English Version

### Overview

`h3_monitor` is a continuous monitoring tool for HTTP/3 endpoints that pushes health check results to Uptime Kuma monitoring system via Push API. It supports monitoring multiple endpoints concurrently with detailed error reporting and response time measurement.

### Key Features

- ✅ **HTTP/3 Protocol Support** - Connection testing using QUIC protocol
- ✅ **Multi-Endpoint Monitoring** - Monitor multiple HTTP/3 targets simultaneously
- ✅ **Uptime Kuma Integration** - Real-time status push via Push API
- ✅ **Command-Line Configuration** - Flexible parameter configuration, no hardcoding
- ✅ **Detailed Error Reporting** - Complete error messages for debugging
- ✅ **Response Time Measurement** - Precise millisecond-level response time
- ✅ **Certificate Fingerprint Extraction** - Optional TLS certificate SHA256 fingerprint extraction
- ✅ **Graceful Shutdown** - SIGINT/SIGTERM signal handling

### Requirements

- Go 1.25.4+
- Uptime Kuma instance (for receiving status pushes)
- HTTP/3 endpoints (target services to monitor)

### Installation

#### Build from Source

```bash
# Clone repository
git clone https://github.com/masx200/uptime-kuma-http3-monitor.git
cd uptime-kuma-http3-monitor

# Build tool
go build -o h3_monitor h3_fingerprint.go

# Or use cross-compilation
GOOS=linux GOARCH=amd64 go build -o h3_monitor-linux-amd64 h3_fingerprint.go
GOOS=windows GOARCH=amd64 go build -o h3_monitor.exe h3_fingerprint.go
GOOS=darwin GOARCH=amd64 go build -o h3_monitor-darwin-amd64 h3_fingerprint.go
```

### Usage

#### 1. Basic Usage - Monitor Single Endpoint

```bash
./h3_monitor \
  --target https://example.com:443 \
  --sni example.com \
  --push-token YOUR_PUSH_TOKEN \
  --kuma-url http://localhost:3001 \
  --interval 60
```

**Parameters:**
- `--target`: HTTP/3 endpoint URL (required)
- `--sni`: TLS SNI server name (required)
- `--push-token`: Uptime Kuma push token (required)
- `--kuma-url`: Uptime Kuma instance URL (default: http://localhost:3001)
- `--interval`: Monitoring interval in seconds (default: 60)
- `--timeout`: Connection timeout in seconds (default: 10)

#### 2. Monitor Multiple Endpoints

```bash
./h3_monitor \
  --target https://endpoint1.com:443 --sni endpoint1.com --push-token TOKEN1 \
  --target https://endpoint2.com:443 --sni endpoint2.com --push-token TOKEN2 \
  --target https://endpoint3.com:443 --sni endpoint3.com --push-token TOKEN3 \
  --interval 60
```

**Note:** If fewer push tokens are provided than endpoints, the last token will be reused.

#### 3. Certificate Fingerprint Only (Backward Compatible)

```bash
./h3_monitor \
  --fingerprint-only \
  --target https://example.com:443 \
  --sni example.com
```

This mode runs once and exits, outputting the certificate SHA256 fingerprint, maintaining compatibility with the original tool.

### Uptime Kuma Configuration

#### Create Push Monitor

1. Add new monitor in Uptime Kuma
2. Select monitor type as **Push**
3. Configure monitor name and description
4. Copy the generated **Push Token**
5. Use this token in command line: `--push-token YOUR_TOKEN`

#### Push API Format

The tool sends requests to Uptime Kuma in the following format:

**Success status:**
```
GET /api/push/YOUR_TOKEN?status=up&ping=245&msg=OK
```

**Failure status:**
```
GET /api/push/YOUR_TOKEN?status=down&msg=dial+timeout%3A+no+connection+established
```

### Command-Line Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `--target` | URL | Yes | None | HTTP/3 endpoint URL (can be specified multiple times) |
| `--sni` | String | Yes | None | TLS SNI server name (can be specified multiple times) |
| `--push-token` | String | Yes* | None | Uptime Kuma push token (can be specified multiple times) |
| `--kuma-url` | URL | No | http://localhost:3001 | Uptime Kuma instance URL |
| `--interval` | Integer | No | 60 | Monitoring interval (seconds) |
| `--timeout` | Integer | No | 10 | HTTP/3 connection timeout (seconds) |
| `--fingerprint-only` | Boolean | No | false | Extract certificate fingerprint only and exit |

*Note: If `--push-token` is not provided, the tool enters fingerprint extraction mode (backward compatible)

### Log Output

The tool uses structured log format with timestamps, log levels, and detailed information:

```
2025-12-25T10:00:00Z [INFO] Starting HTTP/3 monitoring service (interval=60s, timeout=10s)
2025-12-25T10:00:00Z [INFO] Spawning monitor for endpoint "endpoint1.com"
2025-12-25T10:00:01Z [INFO] endpoint="endpoint1.com" status="up" ping=245ms msg="OK"
2025-12-25T10:01:01Z [INFO] endpoint="endpoint1.com" status="up" ping=238ms msg="OK"
2025-12-25T10:02:01Z [ERROR] endpoint="endpoint1.com" status="down" msg="dial timeout: no connection established"
2025-12-25T10:02:01Z [WARN] endpoint="endpoint1.com" push failed: 503 Service Unavailable, retrying...
```

### Docker Deployment

#### Dockerfile Example

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY h3_fingerprint.go go.mod go.sum ./
RUN CGO_ENABLED=0 GOOS=linux go build -o h3_monitor h3_fingerprint.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/h3_monitor .
CMD ["./h3_monitor"]
```

#### docker-compose.yml Example

```yaml
version: '3.8'
services:
  h3-monitor:
    build: .
    container_name: h3_monitor
    restart: unless-stopped
    command:
      - --target
      - https://example.com:443
      - --sni
      - example.com
      - --push-token
      - ${PUSH_TOKEN}
      - --kuma-url
      - http://uptime-kuma:3001
      - --interval
      - "60"
    environment:
      - PUSH_TOKEN=your_token_here
```

Run:
```bash
docker-compose up -d
```

### System Service Configuration (systemd)

Create service file `/etc/systemd/system/h3-monitor.service`:

```ini
[Unit]
Description=HTTP/3 Monitoring Service
After=network.target

[Service]
Type=simple
User=monitoring
WorkingDirectory=/opt/h3-monitor
ExecStart=/opt/h3-monitor/h3_monitor \
  --target https://example.com:443 \
  --sni example.com \
  --push-token YOUR_TOKEN \
  --kuma-url http://localhost:3001 \
  --interval 60

Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable h3-monitor
sudo systemctl start h3-monitor
sudo systemctl status h3-monitor
```

### Troubleshooting

#### Issue: Tool shows "404 Not Found"

**Cause:** Push Token is invalid or the corresponding monitor is disabled

**Solutions:**
1. Check if the monitor is active in Uptime Kuma
2. Verify the Push Token is copied correctly
3. Confirm the monitor type is "Push"

#### Issue: Connection timeout

**Cause:** Target endpoint unreachable or firewall blocking

**Solutions:**
1. Check network connectivity: `curl -v --http3 https://target:port`
2. Verify firewall rules allow UDP traffic (QUIC uses UDP)
3. Increase `--timeout` value

#### Issue: Frequent push failures

**Cause:** Uptime Kuma instance unavailable or network issues

**Solutions:**
1. Check Uptime Kuma service status
2. Verify `--kuma-url` is configured correctly
3. Check specific error messages in logs

### Performance Considerations

- **Memory Usage**: ~50MB base + ~5MB per endpoint
- **CPU Usage**: Minimal when idle, peaks during connection establishment
- **Network Traffic**: ~2-5KB per endpoint per check
- **Recommended**: Monitor maximum 10 endpoints, interval not less than 10 seconds

### Security Recommendations

1. **Protect Push Tokens**
   - Don't use directly in command line (visible in process list)
   - Consider using environment variables (future version support)
   - Limit token permissions to read-only

2. **Network Security**
   - Use HTTPS to connect to Uptime Kuma
   - Disable `InsecureSkipVerify` in production

3. **Log Security**
   - Logs don't contain complete Push Tokens
   - Only show token prefix and suffix

### Development

#### Project Structure

```
.
├── h3_fingerprint.go       # Main program source
├── go.mod                  # Go module dependencies
├── go.sum                  # Dependency checksums
├── openspec/               # Specs and design documents
│   └── changes/
│       └── add-uptime-kuma-http3-monitor/
│           ├── proposal.md     # Proposal overview
│           ├── design.md       # Technical design
│           ├── tasks.md        # Implementation tasks
│           └── specs/          # Capability specs
└── README.md               # This document
```

### Contributing

Issues and Pull Requests are welcome!

1. Fork the repository
2. Create feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to branch (`git push origin feature/AmazingFeature`)
5. Open Pull Request

### License

This project is licensed under the MIT License - see [LICENSE](LICENSE) file for details

### Acknowledgments

- [quic-go](https://github.com/quic-go/quic-go) - Go implementation of QUIC protocol
- [Uptime Kuma](https://github.com/louislam/uptime-kuma) - Excellent self-hosted monitoring tool

### Contact

- Report issues: [GitHub Issues](https://github.com/masx200/uptime-kuma-http3-monitor/issues)
- Discussions: [GitHub Discussions](https://github.com/masx200/uptime-kuma-http3-monitor/discussions)

---

**Note:** This is the monitoring tool based on the proposal in [openspec/changes/add-uptime-kuma-http3-monitor/](openspec/changes/add-uptime-kuma-http3-monitor/). For implementation details, see the design document and task list.
