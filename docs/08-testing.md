# 测试与本机验证

## 本机环境

当前开发机：

- 系统：Windows。
- NVIDIA GPU：NVIDIA GeForce RTX 5060 Ti。
- NVIDIA 驱动：591.74。
- `nvidia-smi`：可用。

已验证命令：

```powershell
nvidia-smi --query-gpu=name,uuid,driver_version,memory.total,memory.used,utilization.gpu,temperature.gpu,power.draw --format=csv,noheader,nounits
```

返回字段包含：

- GPU 型号。
- GPU UUID。
- 驱动版本。
- 总显存。
- 已用显存。
- GPU 利用率。
- 温度。
- 功耗。

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
- 风扇、时钟、P-State、PCIe 链路字段。
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

当前会话中浏览器插件所需的 Node REPL 控制工具未暴露。本机存在 Chrome/Edge 可执行文件，但 headless/CDP 探测未稳定开放调试 target，因此尚未完成真实浏览器截图级验证。已完成的前端验证包括 Vite 构建、服务端静态资源托管和端到端 API 数据返回。

## MVP 验收标准

- 一台 Windows 客户端可以上报 GPU 指标：已通过一次性上报验证。
- 服务端可以展示当前 GPU 状态：已通过 `/api/v1/overview` 验证。
- 服务端可以查询最近 1 小时历史曲线：API 已实现。
- 设备断网上线状态正确变化：逻辑已实现，仍需补自动化验证。
- 服务端低磁盘空间时停止指标写入，并保留 800MiB 空闲空间：逻辑已实现。
- Web 面板在桌面和手机宽度下无明显布局错乱：响应式样式已实现，仍需浏览器截图验证。
