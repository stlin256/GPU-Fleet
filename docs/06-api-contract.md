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
          "memory_total_bytes": 17103331328,
          "memory_used_bytes": 5820645376,
          "utilization_gpu_percent": 100,
          "temperature_celsius": 72,
          "power_draw_watts": 179.73,
          "fan_speed_percent": null,
          "graphics_clock_mhz": null,
          "memory_clock_mhz": null,
          "pstate": null
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

| 状态码 | 含义 |
| --- | --- |
| 202 | 已接收 |
| 400 | 请求格式错误 |
| 401 | 签名错误或设备不存在 |
| 403 | 设备被禁用 |
| 408 | 请求时间戳过期 |
| 409 | nonce 重放 |
| 413 | 请求体过大 |
| 429 | 限流 |
| 507 | 磁盘空间保护，拒绝写入 |

## Web API

Web API 使用 Cookie Session 或管理员 API Token。

核心接口：

```text
POST /api/v1/auth/login
POST /api/v1/auth/logout
GET  /api/v1/overview
GET  /api/v1/devices
GET  /api/v1/devices/{device_id}
GET  /api/v1/devices/{device_id}/gpus
GET  /api/v1/gpus/{gpu_id}/series
GET  /api/v1/gpus/{gpu_id}/processes/latest
GET  /api/v1/stats/gpu-utilization
GET  /api/v1/alerts
POST /api/v1/admin/devices
POST /api/v1/admin/devices/{device_id}/rotate-secret
POST /api/v1/admin/devices/{device_id}/disable
```

管理接口只影响服务端记录和认证状态，不修改客户端本地配置。

