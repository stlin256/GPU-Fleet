# 当前实现说明

本文记录当前仓库中已经落地的实现，避免文档规划和代码状态脱节。

## 已实现

- Go module：`gpufleet`。
- 服务端命令：`cmd/gpufleet-server`。
- 客户端命令：`cmd/gpufleet-agent`。
- Agent 使用 `nvidia-smi` 只读采集 NVIDIA GPU 指标。
- Agent 支持 `--print` 本地采集调试模式。
- Agent 使用 HMAC-SHA256 签名主动上报。
- 服务端校验设备 ID、时间戳、nonce 和签名。
- 服务端拒绝重放 nonce。
- 服务端支持 gzip 请求体，并限制解压后的请求体大小。
- 服务端使用 gzip JSONL 分段文件保存时序指标。
- 服务端写入前按保留期清理旧分段，再检查磁盘保护阈值。
- 服务端默认保留 800MiB 磁盘空闲空间，低于阈值返回 `507`。
- 服务端使用 JSON 文件保存管理员、设备和审计元数据。
- 服务端支持 React/Vite 构建目录托管，并保留内置 HTML 面板作为回退。
- Web 面板支持登录、总览、设备列表、GPU 当前状态、GPU 进程快照和 24 小时统计。
- Agent 支持 GPU 进程快照采集和上报。
- Agent 支持本地离线样本队列，网络失败时缓存 GPU 样本并限制本地队列大小。
- 已提供 Windows Service 和 Linux systemd 安装/卸载脚本。
- 已提供 GPU 利用率统计 API 和最新进程快照 API。
- 已添加静态面板路由测试，覆盖 SPA fallback、API 404 和目录越界防护。

## 当前未实现

- 设备密钥轮换接口。
- VictoriaMetrics 存储适配。
- SQLite 元数据适配。
- 浏览器截图级 UI 验证。

## 运行边界

当前 MVP 不依赖外部数据库，适合先在单机公网服务端上验证链路。若设备数、保留时间或查询复杂度提高，应引入 VictoriaMetrics 作为时序后端。

服务端仍然不提供任何客户端控制能力。管理接口只影响服务端的设备记录和认证状态，不会修改客户端本地配置。
