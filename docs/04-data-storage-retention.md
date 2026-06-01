# 数据存储与磁盘保护

## 存储选型

MVP 使用两类存储：

| 数据类型 | 存储 | 内容 |
| --- | --- | --- |
| 时序指标 | VictoriaMetrics single-node | GPU 利用率、显存、温度、功耗、风扇、时钟等 |
| 元数据 | SQLite | 用户、设备、GPU 静态信息、密钥、告警、审计日志 |

这样比直接上 PostgreSQL/TimescaleDB 更轻，部署时只需要服务端程序、VictoriaMetrics 和一个 SQLite 文件。

## VictoriaMetrics 配置

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

## SQLite 配置

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

GPUFleet 使用三层保护。

### 第一层：数据库保留策略

VictoriaMetrics 通过 `-retentionPeriod` 自动删除超出保留期的数据。

注意：VictoriaMetrics 的保留清理不是每条样本到期后立刻释放空间，而是随分区和后台合并逐步完成。因此服务端仍需要 Disk Guard 做实时保护。

### 第二层：数据库最小空闲空间

VictoriaMetrics 使用：

```text
-storage.minFreeDiskSpaceBytes=838860800
```

当存储目录所在磁盘低于 800MiB 空闲空间时，数据库进入保护状态。

### 第三层：服务端 Disk Guard

GPUFleet Server 每 30 秒检查一次数据盘空闲空间：

- 空闲空间大于 1GiB：正常接收。
- 空闲空间在 800MiB 到 1GiB：接收降级，只接收心跳和关键状态。
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

## 指标命名

建议使用 Prometheus 风格指标名，写入 VictoriaMetrics：

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
