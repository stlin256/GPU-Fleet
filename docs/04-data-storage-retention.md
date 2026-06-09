# 数据存储与磁盘保护

## 存储选型

当前默认实现使用三类零外部服务存储，优先满足部署简单、压缩、长范围查询和空间可回收：

| 数据类型 | 存储 | 内容 |
| --- | --- | --- |
| 时序指标 | gzip JSONL 小时分段文件 | GPU 利用率、显存、温度、功耗、风扇、时钟等 |
| 元数据 | JSON 文件 | 管理员、设备、密钥、会话、服务配置、审计日志 |
| 最新进程快照 | JSON 文件 | 每台设备最近一次 GPU 进程占用 |

这样比直接上 PostgreSQL/TimescaleDB 更轻，服务端只需要一个 Go 二进制和数据目录。VictoriaMetrics 与 SQLite 保留为后续生产增强选项。

## 当前文件布局

默认数据目录为 `data`：

```text
data/
  metadata.json
  processes.json
  metrics/
    samples-YYYYMMDDHH.jsonl.gz
```

时序样本按小时写入 `samples-YYYYMMDDHH.jsonl.gz`。每个分段是 gzip 压缩 JSON Lines，便于追加、审计和按保留期整段删除。服务启动后会加载最新状态、原始索引和 rollup 索引，用于总览、GPU 曲线、统计面板和 30D 长范围查询。

## 空间回收

服务端每次写入新指标前会：

1. 按 `-retention-days` 清理过期 gzip 分段。
2. 检查数据盘空闲空间。
3. 若空闲空间低于 `-min-free-mb`，拒绝新指标写入并返回 `507 Insufficient Storage`。

默认值：

```text
-retention-days 30
-min-free-mb 800
```

这满足“压缩 + 空间回收 + 预留 800MiB”的默认目标。即使拒绝指标写入，服务端仍保持 Web 登录、历史查询、诊断包下载和磁盘状态展示。

## 备份与恢复

Linux 服务端提供数据目录备份与恢复脚本：

- `scripts/backup-server-linux.sh`：默认热备份；设置 `STOP_SERVICE=1` 时做冷备份；输出归档、manifest 和 `.sha256` 校验文件。
- `scripts/restore-server-linux.sh`：必须设置 `CONFIRM_RESTORE=1`；恢复前停止服务，把现有数据目录移到带时间戳的 `.pre-restore` 回滚路径，再恢复归档并启动服务。

备份包含数据目录中的指标、元数据、会话、设备记录和证书文件，应按敏感资料保存。设置页“下载数据库”适合临时导出，正式灾备优先使用脚本备份。

## VictoriaMetrics 增强选项

VictoriaMetrics 推荐仅绑定本机：

```powershell
victoria-metrics.exe `
  -httpListenAddr=127.0.0.1:8428 `
  -storageDataPath=.\data\victoriametrics `
  -retentionPeriod=30d `
  -storage.minFreeDiskSpaceBytes=838860800
```

`838860800` 字节等于 800MiB。

关键能力：

- 时序数据自动压缩存储。
- 通过 `-retentionPeriod` 控制保留时长。
- 通过 `-storage.minFreeDiskSpaceBytes` 在磁盘空闲过低时保护存储目录。

## SQLite 增强选项

SQLite 用于低频元数据，不承载大规模时序样本。

初始化建议：

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;
PRAGMA auto_vacuum = INCREMENTAL;
```

`auto_vacuum` 应在创建业务表之前设置。若已有表和数据，需要设置后执行一次 `VACUUM` 才能完成模式切换。

维护策略：

- 每天清理过期审计日志和告警明细。
- 清理后执行 `PRAGMA incremental_vacuum`。
- 每周在维护窗口执行 `VACUUM`，可配置关闭。
- SQLite 文件建议软限制 200MiB。

## 数据保留策略

默认策略：

| 数据 | 精度 | 默认保留 |
| --- | --- | --- |
| GPU 原始指标 | 10 秒 | 30 天 |
| GPU 进程快照 | 30 秒 | 7 天 |
| Agent 错误事件 | 事件 | 30 天 |
| 审计日志 | 事件 | 180 天 |
| 告警记录 | 事件 | 180 天 |

如果服务端磁盘较小，应优先缩短进程快照和原始指标保留时间。

## 磁盘保护策略

当前默认实现使用服务端内置 Disk Guard。若后续引入 VictoriaMetrics，可形成三层保护。

### 第一层：文件保留策略

gzip JSONL 分段通过 `-retention-days` 删除超出保留期的数据。删除的是整段文件，释放空间明确、直接。

若使用 VictoriaMetrics，它也应配置 `-retentionPeriod`。注意 VictoriaMetrics 的保留清理不是每条样本到期后立刻释放空间，而是随分区和后台合并逐步完成。因此服务端仍需要 Disk Guard 做实时保护。

### 第二层：数据库最小空闲空间

使用 VictoriaMetrics 时配置：

```text
-storage.minFreeDiskSpaceBytes=838860800
```

当存储目录所在磁盘低于 800MiB 空闲空间时，数据库进入保护状态。

### 第三层：服务端 Disk Guard

GPUFleet Server 写入指标前检查数据盘空闲空间：

- 空闲空间大于 `-min-free-mb`：正常接收。
- 空闲空间小于 800MiB：拒绝新指标写入，返回 `507 Insufficient Storage`。

即使拒绝指标写入，服务端仍保持：

- Web 登录。
- 历史数据查询。
- 设备管理。
- 磁盘告警查看。

## Agent 本地队列保护

Agent 也必须限制本地磁盘占用：

```toml
local_queue_max_mb = 128
upload_batch_max_samples = 300
upload_timeout_seconds = 10
```

队列超限时丢弃最旧样本，并记录本地事件。不能因为公网服务端异常导致客户端磁盘写满。

## 后续指标命名

若适配 VictoriaMetrics，建议使用 Prometheus 风格指标名：

```text
gpufleet_gpu_utilization_percent
gpufleet_gpu_memory_used_bytes
gpufleet_gpu_memory_total_bytes
gpufleet_gpu_temperature_celsius
gpufleet_gpu_power_draw_watts
gpufleet_gpu_fan_speed_percent
gpufleet_gpu_clock_graphics_mhz
gpufleet_gpu_clock_memory_mhz
```

基础标签：

```text
device_id
gpu_id
gpu_model
os
agent_version
```

注意控制标签基数。进程 PID、命令行、用户名不应作为长期高频指标标签。
