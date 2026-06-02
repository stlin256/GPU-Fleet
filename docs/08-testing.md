# 测试与本机验证

## 本机环境

当前开发机：

- 系统：Windows。
- NVIDIA GPU：NVIDIA GeForce RTX 5060 Ti。
- NVIDIA 驱动：591.74。
- `nvidia-smi`：可用。

已验证命令：

```powershell
nvidia-smi --query-gpu=index,name,uuid,driver_version,vbios_version,memory.total,memory.used,memory.free,memory.reserved,utilization.gpu,utilization.memory,temperature.gpu,temperature.memory,temperature.gpu.tlimit,power.draw,power.limit,enforced.power.limit,fan.speed,clocks.gr,clocks.mem,clocks.sm,clocks.video,pstate,pcie.link.gen.current,pcie.link.gen.max,pcie.link.width.current,pcie.link.width.max,compute_mode,compute_cap,display_active,display_attached,persistence_mode,driver_model.current,ecc.mode.current,mig.mode.current,clocks_event_reasons.active --format=csv,noheader,nounits
```

返回字段包含：

- GPU 型号。
- GPU UUID。
- 驱动版本。
- VBIOS。
- 总显存、已用显存、空闲显存和保留显存。
- GPU 利用率和显存利用率。
- GPU 温度、温度上限，显存温度在本机不支持时为空。
- 当前功耗、功耗上限和强制功耗上限。
- 风扇、图形/显存/SM/视频时钟、P-State。
- PCIe 当前和最大链路信息。
- Compute 能力、显示状态、驱动模型、ECC/MIG 和时钟限速原因。

文档中不记录完整 GPU UUID，避免把设备唯一标识写入仓库。

## 已完成的本机验证

### 构建和单元检查

```powershell
$env:GOCACHE='F:\project\GPUFleet\.gocache'
go test ./...
go build -o bin\gpufleet-server.exe .\cmd\gpufleet-server
go build -o bin\gpufleet-agent.exe .\cmd\gpufleet-agent
cd web
npm run build
cd ..
```

结果：

- `go test ./...` 通过。
- `gpufleet-server.exe` 构建成功。
- `gpufleet-agent.exe` 构建成功。
- `npm run build` 通过，Vite 仅提示 JS chunk 大于 500kB。
- `internal/server` 已有静态面板路由测试，覆盖 React 入口、静态资源、SPA fallback、API 404 和目录越界防护。
- `internal/server` 已有内置 fallback 面板测试，覆盖服务设置入口、密码更改、端口配置、HTTPS 证书、数据库下载、配置引导、GPU 趋势图、离线蒙版，并禁止旧进度条 UI 回归。
- `internal/server` 覆盖登录短时限流和爆破锁定：连续错误触发 `429`、返回 `Retry-After`、锁定期间正确密码也不能从同源登录、其他来源不受影响、成功登录会清理失败计数。
- `internal/server` 已有设备生命周期和登录限流测试，覆盖创建设备、禁用、启用、轮换密钥、旧密钥失效和新密钥生效。

### 本机采集验证

```powershell
.\bin\gpufleet-agent.exe --print
```

结果包含：

- `NVIDIA GeForce RTX 5060 Ti`
- 驱动版本 `591.74`
- GPU 利用率。
- 显存占用。
- 温度。
- 功耗。
- 风扇、图形/显存/SM/视频时钟、P-State、PCIe 当前和最大链路字段。
- VBIOS、显存空闲/保留、显存利用率、功耗上限、Compute 能力、显示状态和驱动模型。
- ECC/MIG 字段在本机不支持时规范化为空值。
- GPU UUID 以 SHA-256 哈希形式输出，不输出原始 UUID。

### 端到端验证

已在同一个 PowerShell 脚本中完成：

1. 启动 `gpufleet-server.exe` 到 `127.0.0.1:18084`，显式设置 `-web-dir web/dist`。
2. 确认 React `index.html` 返回 `<div id="root"></div>`。
3. 确认构建后的 JS 静态资源返回 `200`。
4. 确认未知 API `/api/v1/unknown` 返回 `404`，不会被 SPA fallback 吞掉。
5. 使用 `gpufleet-agent.exe -once -processes` 上报本机 GPU 指标和进程快照。
6. 登录 Web API。
7. 查询 `/api/v1/overview`、`/api/v1/stats/gpu-utilization?hours=24` 和 `/api/v1/processes/latest`。
8. 主动停止服务端进程。

验证结果：

```json
{
  "react_index_served": true,
  "asset_status": 200,
  "api_404_status": 404,
  "device_count": 1,
  "online_device_count": 1,
  "gpu_count": 1,
  "first_gpu": "NVIDIA GeForce RTX 5060 Ti",
  "stats_count": 1,
  "process_count": 30,
  "disk_status": "ok"
}
```

这证明当前 MVP 已经打通 React 静态面板托管、本机 Agent 采集、HMAC 上报、服务端接收、压缩存储、登录查询、统计 API 和 GPU 进程快照 API。

### 设备生命周期验证

已通过单元测试和本机真实 HTTP 端到端验证覆盖：

1. 登录 Web API。
2. 调用 `POST /api/v1/admin/devices` 创建设备并取得一次性密钥。
3. 使用新设备密钥发送签名心跳，返回 `202`。
4. 调用 `POST /api/v1/admin/devices/{id}/disable` 禁用设备。
5. 使用原密钥继续发送签名心跳，返回 `403`。
6. 调用 `POST /api/v1/admin/devices/{id}/enable` 启用设备。
7. 调用 `POST /api/v1/admin/devices/{id}/rotate-secret` 轮换密钥。
8. 使用旧密钥发送签名心跳，返回 `401`。
9. 使用新密钥发送签名心跳，返回 `202`。

这验证了管理接口只影响服务端认证状态，不向客户端返回命令、配置或可执行动作。

本机端到端验证使用临时服务端 `127.0.0.1:18107`、`web/dist` 静态面板和真实 `gpufleet-agent.exe` 完成。验证结果：

```json
{
  "react_index_served": true,
  "disabled_upload_exit": 1,
  "old_secret_upload_exit": 1,
  "device_count": 2,
  "online_device_count": 1,
  "gpu_count": 1,
  "stats_count": 1,
  "process_count": 30,
  "disk_status": "ok"
}
```

其中 `disabled_upload_exit: 1` 表示禁用设备后真实 Agent 上报被服务端拒绝；`old_secret_upload_exit: 1` 表示密钥轮换后旧密钥失效。随后使用新密钥上报成功。

## Agent 测试计划

### Windows

- 前台运行采集命令：已验证。
- 安装为 Windows Service：脚本已提供，仍需管理员权限环境验证。
- 验证服务重启后自动恢复：待在真实服务环境验证。
- 验证网络断开时本地队列增长：已通过失败上传后排队、恢复后回放清空验证。
- 验证队列超过限制后丢弃旧样本：逻辑已实现，仍需容量压测。
- 验证服务端返回 `507` 时不无限重试同一批数据：逻辑已实现，仍需自动化验证。

### Linux

- 前台运行采集命令。
- 安装为 systemd service。
- 验证无显示器环境。
- 验证驱动未加载时的错误状态。
- 验证多 GPU。
- 验证 MIG/ECC 字段在不支持设备上返回 null。

## 服务端测试计划

- HMAC 签名正确时接收。
- HMAC 签名错误时拒绝。
- 时间戳过期时拒绝。
- nonce 重复时拒绝。
- 请求体过大时拒绝。
- 磁盘低于 800MiB 时拒绝指标写入。
- gzip 请求体解压后大小限制。
- 当前 MVP 压缩分段文件保留清理：写入前执行。
- 静态面板目录越界防护：已用单元测试覆盖。
- 设备创建、禁用、启用和密钥轮换：已用单元测试覆盖。
- 登录限流：已用单元测试覆盖。
- 后续 VictoriaMetrics 不可用时返回可诊断错误。
- 后续 SQLite 锁等待和恢复。

## 前端测试计划

- 桌面端 1440px。
- 平板端 768px。
- 手机端 390px。
- 浅色主题。
- 深色主题。
- 空数据状态。
- 设备离线状态。
- 磁盘保护状态。
- 图表密集数据状态。

当前已使用 `scripts/verify-frontend-chrome.mjs` 完成真实 Chrome headless/CDP 浏览器验证。脚本覆盖密码登录、刷新后 Cookie 会话恢复、GPU Fleet 卡片面板和 2x2 历史趋势图、趋势图悬浮读数、深浅主题切换和刷新持久化、设备管理页、服务设置操作入口、品牌 Logo 与仓库署名、移动端总览、移动端 GPU 页、移动端底部固定导航、扩展 GPU 字段可见性和移动端无横向溢出。

最新验证使用重新编译后的示例服务端 `127.0.0.1:8088`、`web/dist` 静态面板和 `scripts/seed-demo-data.mjs` 演示数据。演示数据包含 4 台设备、5 块 GPU，其中 `rig-dual` 包含 2 块 GPU，`rig-offline` 为离线设备。结果文件位于 `logs/frontend-verify-202606030150-login-guard/result.json`：

```json
{
  "ok": true,
  "screenshots": {
    "desktop_overview": "logs\\frontend-verify-202606030150-login-guard\\desktop-overview.png",
    "desktop_overview_dark": "logs\\frontend-verify-202606030150-login-guard\\desktop-overview-dark.png",
    "desktop_devices": "logs\\frontend-verify-202606030150-login-guard\\desktop-devices.png",
    "desktop_settings": "logs\\frontend-verify-202606030150-login-guard\\desktop-settings.png",
    "mobile_overview": "logs\\frontend-verify-202606030150-login-guard\\mobile-overview.png",
    "mobile_gpu": "logs\\frontend-verify-202606030150-login-guard\\mobile-gpu.png"
  },
  "layout": {
    "width": 394,
    "scrollWidth": 394,
    "cardCount": 5,
    "fleetCardCount": 5,
    "fleetTrendCount": 20,
    "offlineMaskCount": 1,
    "mobileNavButtonCount": 4,
    "mobileNavPosition": "fixed",
    "dualDeviceCardCount": 2,
    "dualDeviceColorMatched": true,
    "distinctDeviceColorCount": 3,
    "sparkTooltipCount": 1,
    "detailTrendCount": 20,
    "meterCount": 0,
    "settingsStatCount": 4,
    "settingsOperationCount": 6,
    "theme": "dark",
    "buttonCount": 7
  }
}
```

该轮命令显式要求 `--min-fleet-cards 5 --require-offline-mask true --require-dual-device true`，因此同时验证了 5 块 GPU 卡片、每卡 4 个历史趋势图、趋势图悬浮读数、离线灰色蒙版、同一设备多 GPU 聚合、同设备 GPU 边框同色、移动端底部固定导航、GPU 详情页无旧进度条、设置页服务状态、操作入口完整性、品牌 Logo 和仓库署名。

当前验证脚本输出：

- `desktop-overview.png`：浅色总览。
- `desktop-overview-dark.png`：深色总览。
- `desktop-devices.png`：设备管理。
- `desktop-settings.png`：服务设置。
- `mobile-overview.png`：移动端 GPU Fleet 卡片面板。
- `mobile-gpu.png`：移动端 GPU 详情。

## MVP 验收标准

- 一台 Windows 客户端可以上报 GPU 指标：已通过一次性上报验证。
- 服务端可以展示当前 GPU 状态：已通过 `/api/v1/overview` 验证。
- 服务端可以查询最近 1 小时历史曲线：API 已实现。
- 设备断网上线状态正确变化：逻辑已实现，仍需补自动化验证。
- 服务端低磁盘空间时停止指标写入，并保留 800MiB 空闲空间：逻辑已实现。
- Web 面板在桌面和手机宽度下无明显布局错乱：已通过 Chrome headless 截图和移动端 `scrollWidth` 验证，覆盖深浅主题、GPU Fleet 卡片面板、GPU 详情趋势图和服务设置页。
