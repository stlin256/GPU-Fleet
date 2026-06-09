# Agent 设计

## 运行方式

Agent 是一个跨平台单文件程序：

- Windows：Windows Service。
- Linux：systemd service。
- 开发调试：前台命令行运行。

Agent 只主动访问服务端，不监听公网端口。

## 配置文件

示例：

```toml
server_url = "https://example.com:8443"
device_id = "device_01H..."
secret = "replace-with-device-secret"

sample_interval_seconds = 10
process_interval_seconds = 30
upload_batch_max_samples = 300
upload_timeout_seconds = 10

local_queue_path = "./agent-queue"
local_queue_max_mb = 128

collect_process_username = false
collect_process_commandline = false
gpu_uuid_mode = "hash"
```

配置文件只在本机修改，服务端不能远程覆盖。

## 采集方式

当前版本使用 `nvidia-smi`：

```powershell
nvidia-smi --query-gpu=index,name,uuid,driver_version,vbios_version,memory.total,memory.used,memory.free,memory.reserved,utilization.gpu,utilization.memory,temperature.gpu,temperature.memory,temperature.gpu.tlimit,power.draw,power.limit,enforced.power.limit,fan.speed,clocks.gr,clocks.mem,clocks.sm,clocks.video,pstate,pcie.link.gen.current,pcie.link.gen.max,pcie.link.width.current,pcie.link.width.max,compute_mode,compute_cap,display_active,display_attached,persistence_mode,driver_model.current,ecc.mode.current,mig.mode.current,clocks_event_reasons.active --format=csv,noheader,nounits
```

如果某个驱动版本不支持上述完整字段，Agent 会自动回退到基础字段集，保证基础采集和上报不中断。不支持的字段，例如部分消费级显卡上的 ECC/MIG，会被规范化为空值。

后续替换或增强为 NVML：

- 更少进程开销。
- 更完整的错误码。
- 更强的跨平台一致性。
- 更容易采集 MIG/ECC/进程信息。

## 上报策略

Agent 批量上报 JSON：

- 正常网络：每 10 秒上报当前样本。
- 弱网络：最多聚合 300 条样本后上报。
- 失败重试：指数退避，最大 5 分钟。
- 请求体压缩：支持 gzip。

## 本地权限

Agent 不应以高权限运行，除非采集环境确实需要。

禁止实现：

- `nvidia-smi -pl`
- `nvidia-smi -lgc`
- `nvidia-smi --gpu-reset`
- 杀进程。
- 修改驱动或 CUDA 配置。

## 采集字段

### GPU 静态字段

- GPU 型号。
- GPU UUID 哈希。
- 显存总量。
- 驱动版本。
- VBIOS 版本。
- Compute 模式和 Compute Capability。
- 显示状态、持久化模式和驱动模型。
- MIG 模式，若设备支持。
- ECC 模式，若设备支持。
- CUDA 版本暂未由当前 `nvidia-smi --query-gpu` 路径采集，后续通过 NVML 或其他只读来源补齐。

### GPU 动态字段

- GPU 利用率。
- 显存已用、空闲、保留和显存利用率。
- GPU 温度、显存温度和温度上限，若设备支持。
- 当前功耗、功耗上限和强制功耗上限。
- 风扇。
- 图形时钟。
- 显存时钟。
- SM 时钟。
- 视频时钟。
- P-State。
- PCIe 当前和最大链路代际、宽度。
- 当前时钟限速原因。

### 进程字段

- PID。
- 进程名。
- GPU 显存占用。
- 用户名，可选。
- 命令行，可选且默认关闭。

## 错误处理

Agent 不能因为单次采集失败退出。

常见状态：

- `nvidia_smi_not_found`
- `driver_not_loaded`
- `no_nvidia_gpu`
- `collection_timeout`
- `upload_timeout`
- `auth_failed`
- `server_insufficient_storage`

错误状态会上报给服务端，也会写入本地日志。
