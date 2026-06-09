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

签名串包含请求方法、路径、`device_id`、时间戳、nonce 和请求体 SHA-256。`device_id` 是 HMAC 输入的一部分，避免同一 secret 被误复用时签名可跨设备重放。

兼容说明：0.1.9 服务端会临时兼容 metadata 中已记录且版本低于 0.1.9 的旧 Agent 签名串，以避免服务端先升级导致旧 Agent 全部离线。该兼容路径只适用于已知旧设备，会写入 `device_auth_legacy_signature` 审计事件；新 Agent、未知设备和未记录版本的设备必须使用包含 `device_id` 的签名串。

## Agent 心跳

```text
POST /api/v1/agent/heartbeat
```

请求：

```json
{
  "agent_version": "0.1.9",
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
  "agent_version": "0.1.9",
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
GET  /api/v1/version
GET  /api/v1/overview
GET  /api/v1/devices
GET  /api/v1/gpus/{gpu_id}/series
GET  /api/v1/guest/status
GET  /api/v1/guest/overview
GET  /api/v1/guest/gpus/{gpu_id}/series
GET  /api/v1/stats/gpu-utilization
GET  /api/v1/energy/summary
GET  /api/v1/processes/latest
POST /api/v1/admin/setup/reopen
POST /api/v1/admin/setup/apply
POST /api/v1/admin/password
POST /api/v1/admin/server-config
POST /api/v1/admin/language
POST /api/v1/admin/certificate
GET  /api/v1/admin/database/download
GET  /api/v1/admin/diagnostics/download
POST /api/v1/admin/guest
GET  /api/v1/admin/guest/visits
POST /api/v1/admin/restart
GET  /api/v1/admin/update/status
POST /api/v1/admin/update/proxy
POST /api/v1/admin/update/apply
GET  /api/v1/admin/update/notice
POST /api/v1/admin/devices
PATCH /api/v1/admin/devices/{device_id}
DELETE /api/v1/admin/devices/{device_id}
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

登录接口防爆破：

- 同一客户端 IP 仍保留 10 次/分钟的短时限流。
- 同一客户端 IP 连续 5 次密码错误后进入递进锁定，初始 5 分钟，重复触发最高 60 分钟。
- 锁定响应使用 `429 Too Many Requests`，并返回 `Retry-After` 响应头和 JSON 字段 `retry_after_seconds`。

示例：

```json
{
  "error": "too many login attempts; retry later",
  "retry_after_seconds": 300
}
```

### 版本与变更记录

```text
GET /api/v1/version
```

该接口需要已登录的 Web Cookie Session。版本信息不公开暴露，避免未认证访问者直接获取服务端版本指纹。

服务端优先读取运行仓库中的 `CHANGELOG.md` 作为结构化变更记录来源；若 `-repo-dir`、工作目录、二进制目录和常见部署目录均无法读取有效 changelog，才回退到 `internal/version` 的内置记录。设置页默认展示当前版本，点击“更多更新记录”会在全屏弹窗中展示完整记录。

响应：

```json
{
  "product": "GPUFleet",
  "version": "0.1.9",
  "commit": "dev",
  "author": "stlin256",
  "repository": "https://github.com/stlin256/GPU-Fleet",
  "changelog": [
    {
      "version": "0.1.9",
      "date": "2026-06-09",
      "title": "运行诊断与长期数据查询强化",
      "added": [
        "设置页新增只读诊断包下载，导出版本、运行时、磁盘、设备、GPU、进程、更新缓存和最近审计摘要，并脱敏代理凭据和远端 IP。"
      ],
      "changed": [
        "前端 Chrome/CDP 验证脚本补充诊断包入口、关键设置弹窗和截图非空检查，并支持显式期望版本参数。"
      ]
    }
  ]
}
```

服务端和 Agent 同时支持 `-version` 命令行参数，发布构建可通过 Go `-ldflags` 注入 `commit` 和 `build_time`。

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
GET  /api/v1/admin/diagnostics/download
GET  /api/v1/admin/update/status
POST /api/v1/admin/update/proxy
POST /api/v1/admin/update/apply
GET  /api/v1/admin/update/notice
POST /api/v1/admin/guest
GET  /api/v1/admin/guest/visits
POST /api/v1/admin/restart
```

- `GET /setup/status`：公开状态探测，返回是否需要首次配置、当前监听协议、配置端口、界面语言、HTTPS 证书状态和是否需要重启。
- `POST /setup/apply`：仅在尚无密码的首次部署可用，用于设置访问密码、端口、界面语言和可选证书。
- `POST /admin/setup/reopen` 与 `/admin/setup/apply`：登录后再次打开并应用配置引导，可集中调整密码、端口、界面语言和证书。
- `POST /admin/password`：修改 Web 访问密码。
- `POST /admin/server-config`：保存访问端口、磁盘预留空间、自动更新开关和能耗展示参数。端口不会热切换，响应会标记是否需要重启；磁盘预留空间、自动更新开关和能耗展示参数立即生效。
- `POST /admin/language`：保存界面语言；当前支持 `zh-CN` 和 `en-US`，即时生效且不需要重启。
- `POST /admin/certificate`：上传证书 PEM 和私钥 PEM；无证书使用 HTTP，证书保存后服务端会调度自动重启，恢复后使用 HTTPS。
- `GET /admin/database/download`：下载运行数据库压缩包，仅包含 `metadata.json`、`processes.json` 和 `metrics/`，不包含证书私钥。
- `GET /admin/diagnostics/download`：下载只读诊断 ZIP，包含 `diagnostics.json`，汇总版本、运行时、磁盘、指标分段、设备、GPU、进程、更新缓存和最近审计摘要，并脱敏代理凭据和远端 IP。
- `GET /admin/update/status`：检查服务端自身 Git 工作区和 upstream 状态。
- `POST /admin/update/proxy`：保存或清空在线更新代理地址，仅接受 `http` 或 `https` 代理；诊断包中会脱敏代理凭据。
- `POST /admin/update/apply`：仅在工作区干净、存在 upstream、本地未超前且可 fast-forward 时预检依赖、构建远端提交、执行 `git pull --ff-only`，并安排服务端自动重启。
- `GET /admin/update/notice`：返回并清除一条自动更新完成通知。通知包含更新时间、提交、版本和中英文更新内容；同版本更新只返回新旧 changelog 顶部同版本条目中新增或变化的行。
- `POST /admin/guest`：开启或关闭访客入口和访客总览。
- `GET /admin/guest/visits`：返回最近 100 次访客总览访问记录，包含远端地址、User-Agent、语言、平台、屏幕、时区和浏览器指纹摘要。
- `POST /admin/restart`：手动调度服务端重启。页面会全屏等待服务恢复，恢复后刷新并显示完成提示。

在线更新接口只接受固定路径，不读取请求体参数，不允许前端传入命令、远端、分支或仓库路径。更新前会检查 `git`、`go`、Windows 的 `powershell.exe` 或 Linux 的 `/bin/sh`、服务端源码入口和当前可执行文件目录写入权限。依赖缺失时返回错误且不会开始应用更新。更新状态在前端缓存 1 小时，管理员可手动重新检查；设置页可保存代理地址，后端 Git 和 Go 构建会复用该代理环境。自动更新默认开启，每 30 分钟执行同一套服务端检查和应用逻辑；旧 metadata 没有 `auto_update_enabled` 字段时按开启处理。

`POST /admin/server-config` 可额外接收以下能耗展示字段，这些字段只影响 Web 展示、估算和诊断口径，不会下发到 Agent，也不会修改 GPU 功耗、风扇或频率：

```json
{
  "energy_price_per_kwh": 0.75,
  "energy_currency": "CNY",
  "thermal_hot_celsius": 85,
  "idle_utilization_percent": 5,
  "idle_power_watts": 30
}
```

校验规则：

- `energy_price_per_kwh` 必须大于等于 `0`。
- `energy_currency` 为空时默认为 `CNY`，最长 12 个字符，仅允许字母、数字、`-` 和 `_`。
- `thermal_hot_celsius` 范围为 `1-120`。
- `idle_utilization_percent` 范围为 `0-100`。
- `idle_power_watts` 范围为 `0-2000`。

更新状态响应示例：

```json
{
  "available": true,
  "supported": true,
  "dirty": false,
  "branch": "main",
  "remote": "https://github.com/stlin256/GPU-Fleet.git",
  "upstream": "origin/main",
  "local_commit": "1111111111111111111111111111111111111111",
  "remote_commit": "2222222222222222222222222222222222222222",
  "running_version": "0.1.9",
  "running_commit": "1111111111111111111111111111111111111111",
  "repo_version": "0.1.9",
  "binary_outdated": false,
  "behind": 1,
  "ahead": 0,
  "supply_chain": {
    "ok": true,
    "blocked": false,
    "remote_trusted": true,
    "remote_kind": "network",
    "remote_host": "github.com",
    "remote_repository": "stlin256/gpu-fleet",
    "upstream_bound": true,
    "fast_forward_only": true,
    "worktree_clean": true,
    "exact_target_commit": true
  },
  "checked_at": "2026-06-05T12:00:00Z",
  "message": "update available"
}
```

应用更新响应示例：

```json
{
  "ok": true,
  "target_commit": "2222222222222222222222222222222222222222",
  "restart_required": true,
  "restarting": true,
  "restart_at": "2026-06-05T12:00:07Z",
  "output": "Updating 1111111..2222222\nFast-forward",
  "build_output": "",
  "dependency_status": {
    "ok": true,
    "platform": "windows",
    "checked": ["repo-dir", "git", "go", "./cmd/gpufleet-server", "current executable", "executable directory writable", "powershell.exe"]
  },
  "status": {
    "available": false,
    "supported": true,
    "dirty": false,
    "branch": "main",
    "upstream": "origin/main",
    "local_commit": "2222222222222222222222222222222222222222",
    "remote_commit": "2222222222222222222222222222222222222222",
    "behind": 0,
    "ahead": 0,
    "checked_at": "2026-06-05T12:00:05Z",
    "message": "already up to date"
  }
}
```

### 能耗与热状态摘要

```text
GET /api/v1/energy/summary?hours=24
```

该接口需要已登录的 Web Cookie Session。`hours` 支持 `1-720`，前端当前使用 `24`、`168` 和 `720`。服务端基于现有 GPU 时序点派生能耗和热状态：

```text
kWh = Σ 平均功率 W × 时间差 h / 1000
```

当相邻采样间隔超过当前范围的保护上限时，该段不累计能耗，避免设备离线或数据缺口被误算成持续耗电。1H 使用原始序列，24H 使用分钟 rollup，7D/30D 使用小时 rollup。

响应示例：

```json
{
  "hours": 24,
  "since": "2026-06-08T12:00:00Z",
  "until": "2026-06-09T12:00:00Z",
  "config": {
    "energy_price_per_kwh": 0.75,
    "energy_currency": "CNY",
    "thermal_hot_celsius": 85,
    "idle_utilization_percent": 5,
    "idle_power_watts": 30
  },
  "summary": {
    "current_power_watts": 2400,
    "average_power_watts": 2300,
    "peak_power_watts": 2600,
    "energy_kwh": 55.2,
    "estimated_cost": 41.4,
    "currency": "CNY",
    "hot_gpu_count": 2,
    "throttled_gpu_count": 1,
    "high_idle_power_gpu_count": 1,
    "idle_waste_kwh": 2.3,
    "coverage_percent": 96,
    "sample_count": 1800,
    "power_sample_count": 1800
  },
  "series": [
    {
      "timestamp": "2026-06-09T11:59:00Z",
      "power_watts": 2400,
      "peak_temperature_celsius": 82,
      "hot_gpu_count": 0,
      "gpu_sample_count": 8
    }
  ],
  "gpus": [
    {
      "device_id": "device_20260609120000",
      "device_alias": "worker-a100-01",
      "gpu_id": "0",
      "gpu_name": "NVIDIA A100",
      "sample_count": 240,
      "power_sample_count": 240,
      "current_power_watts": 285,
      "average_power_watts": 260,
      "peak_power_watts": 310,
      "peak_temperature_celsius": 86,
      "energy_kwh": 6.2,
      "estimated_cost": 4.65,
      "hot_sample_count": 3,
      "hot_seconds": 120,
      "throttled": true,
      "throttle_reason": "HW Slowdown",
      "idle_waste_kwh": 0.3,
      "high_idle_power_seconds": 3600,
      "coverage_percent": 97
    }
  ],
  "diagnostics": [
    {
      "kind": "thermal",
      "severity": "warning",
      "device_id": "device_20260609120000",
      "device_alias": "worker-a100-01",
      "gpu_id": "0",
      "gpu_name": "NVIDIA A100",
      "value": 86,
      "unit": "celsius"
    }
  ]
}
```

`diagnostics.kind` 当前可能为：

- `thermal`：峰值温度达到高温阈值。
- `throttle`：最新快照存在有效时钟限速原因。
- `idle_waste`：低利用率同时功率高于空转功率阈值。

该接口是只读展示接口，不提供功耗墙、风扇、频率、任务暂停或其他客户端控制能力。

### 访客接口

访客接口不需要 Web Cookie Session，但只有在管理员开启访客功能后才可访问。关闭访客功能后，访客状态以外的访客接口返回 `403`。

```text
GET /api/v1/guest/status
GET /api/v1/guest/overview
GET /api/v1/guest/gpus/{gpu_id}/series?device_id=guest-device-1&hours=1
```

- `GET /guest/status`：返回访客功能是否开启、当前语言和协议等最小状态，用于登录页决定是否显示访客入口。
- `GET /guest/overview`：返回脱敏总览。响应不包含 GPU 进程、24 小时统计、真实设备 ID、主机名、OS、Agent 版本、最近错误、GPU UUID、驱动版本和 VBIOS。
- `GET /guest/gpus/{gpu_id}/series`：只接受访客总览中返回的匿名设备 ID，例如 `guest-device-1`。服务端会在访客功能开启时映射到真实设备，并在关闭后拒绝读取。

前端会发送以下可选头部用于访客记录和浏览器指纹摘要：

```text
X-GPUFleet-Guest-Fingerprint
X-GPUFleet-Guest-Language
X-GPUFleet-Guest-Platform
X-GPUFleet-Guest-Screen
X-GPUFleet-Guest-Timezone
```

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

### 修改、删除、启用、禁用和轮换密钥

```text
PATCH  /api/v1/admin/devices/{device_id}
DELETE /api/v1/admin/devices/{device_id}
POST /api/v1/admin/devices/{device_id}/enable
POST /api/v1/admin/devices/{device_id}/disable
POST /api/v1/admin/devices/{device_id}/rotate-secret
```

- `PATCH`：修改设备别名；空别名会回退为设备 ID，最长 96 个字符。
- `DELETE`：删除服务端设备记录，并清理该设备的最新 GPU 缓存和最新进程快照；删除后原密钥立即失效。
- `enable`：允许该设备继续通过现有密钥上报。
- `disable`：服务端拒绝该设备后续上报，返回 `403`。
- `rotate-secret`：生成新密钥；旧密钥立即失效。

这些操作只改变服务端记录和认证状态。Agent 仍由本机管理员维护配置，服务端不会远程改写 Agent 配置。
