# API 契约

## 版本

所有 API 使用版本前缀：

```text
/api/v1
```

## Agent 认证头

```text
X-GF-Device-Id: device_01H...
X-GF-Timestamp: 2026-06-01T12:00:00Z
X-GF-Nonce: random-128-bit
X-GF-Signature: base64-hmac-sha256
Content-Encoding: gzip
```

## Agent 心跳

```text
POST /api/v1/agent/heartbeat
```

请求：

```json
{
  "agent_version": "0.1.0",
  "hostname": "workstation-01",
  "os": "windows",
  "os_version": "Windows 11",
  "boot_time": "2026-06-01T08:00:00Z",
  "gpu_count": 1,
  "timestamp": "2026-06-01T12:00:00Z"
}
```

响应：

```json
{
  "accepted": true,
  "server_time": "2026-06-01T12:00:01Z"
}
```

## GPU 样本上报

```text
POST /api/v1/agent/samples
```

请求：

```json
{
  "device_id": "device_01H...",
  "agent_version": "0.1.0",
  "samples": [
    {
      "timestamp": "2026-06-01T12:00:00Z",
      "gpus": [
        {
          "gpu_id": "gpu0",
          "uuid_hash": "sha256:...",
          "name": "NVIDIA GeForce RTX 5060 Ti",
          "driver_version": "591.74",
          "vbios_version": "98.06.1f.00.c8",
          "memory_total_bytes": 17103331328,
          "memory_used_bytes": 5820645376,
          "memory_free_bytes": 11315167232,
          "memory_reserved_bytes": 273678336,
          "utilization_gpu_percent": 100,
          "utilization_memory_percent": 70,
          "temperature_celsius": 72,
          "temperature_memory_celsius": null,
          "temperature_limit_celsius": 15,
          "power_draw_watts": 179.73,
          "power_limit_watts": 180,
          "power_enforced_limit_watts": 180,
          "fan_speed_percent": 84,
          "graphics_clock_mhz": 2602,
          "memory_clock_mhz": 16001,
          "sm_clock_mhz": 2602,
          "video_clock_mhz": 2265,
          "pstate": "P0",
          "pcie_link_generation": "3",
          "pcie_link_width": "8",
          "pcie_link_generation_max": "3",
          "pcie_link_width_max": "16",
          "compute_mode": "Default",
          "compute_capability": "12.0",
          "display_active": "Disabled",
          "display_attached": "No",
          "persistence_mode": null,
          "driver_model": "WDDM",
          "ecc_mode_current": null,
          "mig_mode_current": null,
          "clock_throttle_reasons": "0x0000000000000000"
        }
      ]
    }
  ]
}
```

响应：

```json
{
  "accepted": true,
  "accepted_samples": 1
}
```

## 进程快照上报

```text
POST /api/v1/agent/process-snapshots
```

请求：

```json
{
  "device_id": "device_01H...",
  "timestamp": "2026-06-01T12:00:00Z",
  "processes": [
    {
      "gpu_id": "gpu0",
      "pid": 1234,
      "process_name": "python.exe",
      "used_memory_bytes": 2147483648,
      "username": null,
      "commandline": null
    }
  ]
}
```

## 服务端错误码

当前实现：

| 状态码 | 含义 |
| --- | --- |
| 202 | 已接收 |
| 400 | 请求格式错误 |
| 401 | 签名错误或设备不存在 |
| 403 | 设备被禁用 |
| 409 | nonce 重放 |
| 413 | 请求体过大 |
| 429 | 登录或 Agent 上报触发限流 |
| 507 | 磁盘空间保护，拒绝写入 |

预留增强：

| 状态码 | 含义 |
| --- | --- |
| 408 | 请求时间戳过期，当前实现归入 `401` |

## Web API

Web API 当前使用 Cookie Session。

已实现接口：

```text
GET  /api/v1/setup/status
POST /api/v1/setup/apply
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/overview
GET  /api/v1/devices
GET  /api/v1/gpus/{gpu_id}/series
GET  /api/v1/stats/gpu-utilization
GET  /api/v1/processes/latest
POST /api/v1/admin/setup/reopen
POST /api/v1/admin/setup/apply
POST /api/v1/admin/password
POST /api/v1/admin/server-config
POST /api/v1/admin/certificate
GET  /api/v1/admin/database/download
POST /api/v1/admin/devices
POST /api/v1/admin/devices/{device_id}/enable
POST /api/v1/admin/devices/{device_id}/disable
POST /api/v1/admin/devices/{device_id}/rotate-secret
```

规划中接口：

```text
GET  /api/v1/devices/{device_id}
GET  /api/v1/devices/{device_id}/gpus
GET  /api/v1/gpus/{gpu_id}/processes/latest
GET  /api/v1/alerts
```

登录只使用密码作为凭据，不需要账户名。登录成功后服务端签发记住当前浏览器设备的 Cookie，会话 30 天后过期；服务端重启后仍可识别未过期会话。管理接口只影响服务端记录和认证状态，不修改客户端本地配置。

### 首次配置和服务设置

```text
GET  /api/v1/setup/status
POST /api/v1/setup/apply
POST /api/v1/admin/setup/reopen
POST /api/v1/admin/setup/apply
POST /api/v1/admin/password
POST /api/v1/admin/server-config
POST /api/v1/admin/certificate
GET  /api/v1/admin/database/download
```

- `GET /setup/status`：公开状态探测，返回是否需要首次配置、当前监听协议、配置端口、HTTPS 证书状态和是否需要重启。
- `POST /setup/apply`：仅在尚无密码的首次部署可用，用于设置访问密码、端口和可选证书。
- `POST /admin/setup/reopen` 与 `/admin/setup/apply`：登录后再次打开并应用配置引导。
- `POST /admin/password`：修改 Web 访问密码。
- `POST /admin/server-config`：保存访问端口；当前进程不会热切换端口，响应会标记是否需要重启。
- `POST /admin/certificate`：上传证书 PEM 和私钥 PEM；无证书使用 HTTP，有证书并重启后使用 HTTPS。
- `GET /admin/database/download`：下载运行数据库压缩包，仅包含 `metadata.json`、`processes.json` 和 `metrics/`，不包含证书私钥。

### 创建设备

```text
POST /api/v1/admin/devices
```

请求：

```json
{
  "alias": "worker-a100-01"
}
```

响应：

```json
{
  "device": {
    "id": "device_20260602120000",
    "alias": "worker-a100-01",
    "enabled": true
  },
  "secret": "one-time-device-secret"
}
```

设备密钥只在创建或轮换接口响应中返回一次。服务端保存密钥用于校验 Agent HMAC，上报响应不会返回任何命令、配置或可执行动作。

### 启用、禁用和轮换密钥

```text
POST /api/v1/admin/devices/{device_id}/enable
POST /api/v1/admin/devices/{device_id}/disable
POST /api/v1/admin/devices/{device_id}/rotate-secret
```

- `enable`：允许该设备继续通过现有密钥上报。
- `disable`：服务端拒绝该设备后续上报，返回 `403`。
- `rotate-secret`：生成新密钥；旧密钥立即失效。

这些操作只改变服务端认证记录。Agent 仍由本机管理员维护配置，服务端不会远程改写 Agent 配置。
