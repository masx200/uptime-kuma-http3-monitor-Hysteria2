# sing-box 配置文件文档

## 概述

`config.json` 是 sing-box
代理服务器的核心配置文件，定义了多种代理协议的入站连接、出站连接以及路由规则。该配置支持
Hysteria2、VLESS (Reality) 协议，并提供灵活的流量路由功能。

## 配置结构

```json
{
  "log": {},
  "inbounds": [],
  "outbounds": [],
  "route": {}
}
```

## 配置详情

### 1. 日志配置 (Log)

```json
"log": {
  "disabled": false,
  "level": "info"
}
```

| 参数       | 类型    | 默认值 | 描述                                                            |
| ---------- | ------- | ------ | --------------------------------------------------------------- |
| `disabled` | boolean | false  | 是否禁用日志记录                                                |
| `level`    | string  | "info" | 日志级别，可选值：trace, debug, info, warn, error, fatal, panic |

### 2. 入站连接 (Inbounds)

配置支持两种代理协议，共享同一个端口 20143：

#### 2.1 Hysteria2 协议

```json
{
  "type": "hysteria2",
  "listen": "::",
  "listen_port": 20143,
  "users": [
    {
      "password": "4519a3fa-b2f4-4edc-9fa2-7f9433110665"
    }
  ],
  "masquerade": "https://www.bing.com",
  "tls": {
    "enabled": true,
    "alpn": ["h3"],
    "certificate_path": "/home/container/.npm/cert.pem",
    "key_path": "/home/container/.npm/private.key"
  }
}
```

| 参数          | 类型   | 描述                                          |
| ------------- | ------ | --------------------------------------------- |
| `type`        | string | 协议类型，固定为 "hysteria2"                  |
| `listen`      | string | 监听地址，"::" 表示监听所有 IPv6 和 IPv4 地址 |
| `listen_port` | number | 监听端口                                      |
| `users`       | array  | 用户认证信息                                  |
| `password`    | string | Hysteria2 协议密码，通常使用 UUID 格式        |
| `masquerade`  | string | 伪装网站 URL，用于流量混淆                    |
| `tls`         | object | TLS 加密配置                                  |

#### 2.2 VLESS (Reality) 协议

```json
{
  "type": "vless",
  "listen": "::",
  "listen_port": 20143,
  "users": [
    {
      "uuid": "4519a3fa-b2f4-4edc-9fa2-7f9433110665",
      "flow": "xtls-rprx-vision"
    }
  ],
  "tls": {
    "enabled": true,
    "server_name": "www.nazhumi.com",
    "reality": {
      "enabled": true,
      "handshake": {
        "server": "www.nazhumi.com",
        "server_port": 443
      },
      "private_key": "8IBXlzDxxj8heJzkFpgjOwMK0vRVytxuUUl2SCqCwWM",
      "short_id": [""]
    }
  }
}
```

| 参数                    | 类型   | 描述                                           |
| ----------------------- | ------ | ---------------------------------------------- |
| `type`                  | string | 协议类型，固定为 "vless"                       |
| `uuid`                  | string | 用户唯一标识符                                 |
| `flow`                  | string | 流控模式，"xtls-rprx-vision" 支持 TCP 流量伪装 |
| `server_name`           | string | TLS 服务器名称，用于伪装                       |
| `reality`               | object | Reality 协议配置                               |
| `handshake.server`      | string | 握手目标服务器                                 |
| `handshake.server_port` | number | 握手目标端口                                   |
| `private_key`           | string | Reality 私钥                                   |
| `short_id`              | array  | 短 ID 列表，用于客户端连接                     |

### 3. 出站连接 (Outbounds)

配置了两个出站连接：

#### 3.1 直连

```json
{
  "type": "direct"
}
```

直接连接到目标服务器，不经过任何代理。

#### 3.2 SOCKS5 代理

```json
{
  "type": "socks",
  "tag": "SOCKS5-PROXY",
  "server": "127.0.0.1",
  "server_port": 1080,
  "version": "5",
  "username": "g7envpwz14b0u55",
  "password": "juvytdsdzc225pq"
}
```

| 参数          | 类型   | 描述                       |
| ------------- | ------ | -------------------------- |
| `type`        | string | 出站类型，"socks"          |
| `tag`         | string | 出站标签，用于路由规则引用 |
| `server`      | string | SOCKS5 代理服务器地址      |
| `server_port` | number | SOCKS5 代理服务器端口      |
| `version`     | string | SOCKS 协议版本             |
| `username`    | string | SOCKS5 认证用户名          |
| `password`    | string | SOCKS5 认证密码            |

### 4. 路由规则 (Route)

```json
"route": {
  "rules": [
    {
      "domain": [".*"],
      "outbound": "SOCKS5-PROXY"
    },
    {
      "protocol": "dns",
      "outbound": "SOCKS5-PROXY"
    }
  ],
  "final": "SOCKS5-PROXY"
}
```

#### 4.1 路由规则 (Rules)

| 规则类型 | 匹配条件          | 目标出站     | 描述              |
| -------- | ----------------- | ------------ | ----------------- |
| 域名规则 | `domain: [".*"]`  | SOCKS5-PROXY | 匹配所有域名流量  |
| 协议规则 | `protocol: "dns"` | SOCKS5-PROXY | 匹配 DNS 查询流量 |

#### 4.2 默认路由 (Final)

```json
"final": "SOCKS5-PROXY"
```

所有未被规则明确匹配的流量都将通过 SOCKS5-PROXY 出站。

## 安全特性

### 1. TLS 加密

- 所有入站连接都启用了 TLS 加密
- 使用自签名证书存储在 `.npm/` 目录
- Hysteria2 使用 ALPN h3 进行 HTTP/3 伪装

### 2. Reality 协议

- VLESS 协议使用 Reality 进行流量伪装
- 伪装成 `www.nazhumi.com` 的 HTTPS 流量
- 通过私钥/公钥对进行身份验证

### 3. 认证机制

- Hysteria2 使用密码认证
- VLESS 使用 UUID 认证
- SOCKS5 代理使用用户名/密码认证

## 端口使用情况

| 端口  | 协议    | 用途                        |
| ----- | ------- | --------------------------- |
| 20143 | TCP/UDP | Hysteria2 和 VLESS 协议监听 |
| 1080  | TCP     | SOCKS5 代理服务器           |

## 流量处理流程

1. **入站流量** → 客户端通过 Hysteria2 或 VLESS 协议连接到 20143 端口
2. **路由匹配** → 根据路由规则匹配流量类型
3. **出站转发** → 匹配的流量通过 SOCKS5-PROXY (127.0.0.1:1080) 转发
4. **最终路由** → 未匹配的流量也通过 SOCKS5-PROXY 处理

## 配置维护

### 证书更新

- 证书文件路径：`/home/container/.npm/cert.pem`
- 私钥文件路径：`/home/container/.npm/private.key`
- 建议定期更新证书以确保安全性

### 密钥轮换

- UUID 和 Reality 私钥存储在 `.npm/` 目录
- 可通过 `start.sh` 脚本重新生成
- 更新后需要同步客户端配置

### 监控建议

- 监听端口 20143 的连接状态
- 检查 SOCKS5 代理 127.0.0.1:1080 的可用性
- 关注日志中的错误和警告信息

## 故障排除

### 常见问题

1. **端口冲突**：确保 20143 和 1080 端口未被其他进程占用
2. **证书问题**：检查证书文件是否存在且权限正确
3. **路由失效**：验证 SOCKS5 代理服务器是否正常运行
4. **连接失败**：检查防火墙设置和网络连接

### 调试命令

```bash
# 检查配置文件语法
.sing-box check -c config.json

# 查看实时日志
.sing-box run -c config.json --log-level debug

# 测试端口监听
netstat -tulpn | grep 20143
```

## 配置优化建议

1. **性能优化**：根据服务器负载调整日志级别
2. **安全加固**：使用更复杂的密码和定期轮换密钥
3. **路由细化**：添加更精确的路由规则以提高性能
4. **监控告警**：集成日志监控和性能指标收集
