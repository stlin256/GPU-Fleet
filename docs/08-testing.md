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

## Agent 测试计划

### Windows

- 前台运行采集命令。
- 安装为 Windows Service。
- 验证服务重启后自动恢复。
- 验证网络断开时本地队列增长。
- 验证队列超过限制后丢弃旧样本。
- 验证服务端返回 `507` 时不无限重试同一批数据。

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
- VictoriaMetrics 不可用时返回可诊断错误。
- SQLite 锁等待和恢复。

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

## MVP 验收标准

- 一台 Windows 客户端可以持续上报 GPU 指标。
- 服务端可以展示当前 GPU 状态。
- 服务端可以查询最近 1 小时历史曲线。
- 设备断网上线状态正确变化。
- 服务端低磁盘空间时停止指标写入，并保留 800MiB 空闲空间。
- Web 面板在桌面和手机宽度下无明显布局错乱。

