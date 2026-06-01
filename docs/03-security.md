# 安全设计

## 安全目标

- 防止伪造客户端上报。
- 防止请求重放。
- 防止服务端成为远程控制通道。
- 防止敏感主机信息默认泄露。
- 防止数据库直接暴露公网。
- 防止磁盘写满导致服务端不可用。

## 服务端不能影响客户端

GPUFleet 不提供以下 API：

- 下发命令。
- 下发配置。
- 下发脚本。
- 远程升级 Agent。
- 修改 GPU 设置。
- 杀进程或重启进程。
- 远程读取客户端文件。

Agent 请求服务端后，服务端只返回接收结果，不返回可执行动作。

允许的响应示例：

```json
{
  "accepted": true,
  "server_time": "2026-06-01T12:00:00Z"
}
```

不允许的响应示例：

```json
{
  "next_config": {},
  "commands": []
}
```

## Agent 身份认证

MVP 推荐 HMAC-SHA256 请求签名。

请求头：

```text
X-GF-Device-Id: device_01H...
X-GF-Timestamp: 2026-06-01T12:00:00Z
X-GF-Nonce: random-128-bit
X-GF-Signature: base64(hmac_sha256(secret, signing_string))
```

签名串：

```text
METHOD + "\n" +
PATH + "\n" +
TIMESTAMP + "\n" +
NONCE + "\n" +
SHA256_HEX(BODY)
```

服务端校验：

- `device_id` 存在且启用。
- 时间戳偏差不超过 5 分钟。
- nonce 在 10 分钟内未出现过。
- 签名匹配。
- body 大小不超过限制。

后续增强可支持 mTLS，每台设备使用独立客户端证书。

## Web 认证

MVP 支持本地管理员账号：

- 密码使用 Argon2id 或 bcrypt 哈希。
- Cookie 使用 `HttpOnly`、`Secure`、`SameSite=Lax`。
- 登录失败限流。
- 管理员可创建、禁用、轮换设备密钥。

## 数据脱敏

默认采集：

- GPU 型号。
- 驱动版本。
- 显存、温度、功耗、利用率。
- GPU UUID 的哈希值。
- 设备别名。

默认不采集：

- 操作系统用户名。
- 完整进程命令行。
- 环境变量。
- 文件路径。
- 私网 IP 列表。

如确实需要采集用户名或命令行，只能在 Agent 本地配置中显式开启。服务端不能远程开启。

## 网络安全

- 生产环境必须使用 HTTPS。
- VictoriaMetrics 只监听 `127.0.0.1` 或内网地址。
- SQLite 文件不暴露网络访问。
- 服务端开启请求体大小限制。
- Agent 上报接口独立限流。

## 审计

SQLite 中记录以下审计事件：

- Web 登录成功/失败。
- 创建、禁用、轮换设备密钥。
- 设备首次注册。
- 设备认证失败。
- 磁盘保护触发。
- 数据库写入失败。

