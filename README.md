# GPUFleet

GPUFleet 是一个用于聚合多台 Windows/Linux 设备上 NVIDIA GPU 运行信息的客户端-服务端系统。

核心目标：

- 客户端 Agent 只做本机只读采集，主动向公网服务端上报。
- 服务端只接收、存储、统计和展示数据，不向客户端下发命令。
- 依赖保持简单，MVP 不引入 Kubernetes、Kafka、Prometheus、Grafana。
- 时序数据具备压缩、保留策略和磁盘保护能力，默认预留 800MiB 空闲空间。
- Web 面板适配桌面、平板和移动端，使用现代组件库构建。

## 当前可运行版本

当前仓库已经包含一个不依赖外部服务的 MVP：

- `gpufleet-server`：公网服务端、HMAC Agent 接入、压缩指标存储、Web 面板和 API。
- `gpufleet-agent`：Windows/Linux 客户端，使用 `nvidia-smi` 只读采集 NVIDIA GPU 指标并主动上报，当前包含显存、利用率、温度、功耗、风扇、时钟、PCIe、VBIOS、Compute、ECC/MIG 等运行字段；不支持的字段会留空。
- Web 面板：React + Vite + TypeScript，使用 lucide-react、TanStack Query 和 Apache ECharts，首页优先展示多机多卡 GPU Fleet 聚合面板，支持深浅主题切换，构建产物已纳入 `web/dist`。
- 设备管理：Web/API 支持创建设备、显示一次性密钥、禁用/启用设备和轮换密钥。
- 存储：标准库实现的 gzip JSONL 分段时序文件 + JSON 元数据文件。
- 磁盘保护：服务端写入前清理过期压缩分段，默认至少保留 800MiB 空闲空间，低于阈值时拒绝新指标写入。
- 安全边界：服务端不提供命令下发、配置下发、GPU 设置修改或远程执行接口。
- 基础防护：登录按客户端 IP 限流，Agent 上报按客户端 IP + 设备 ID 限流。

这版先保证端到端可运行和依赖简单。后续如需要更强的长期查询能力，可以把指标存储适配到 VictoriaMetrics single-node，元数据适配到 SQLite。

## 快速运行

构建：

```powershell
$env:GOCACHE='F:\project\GPUFleet\.gocache'
go build -o bin\gpufleet-server.exe .\cmd\gpufleet-server
go build -o bin\gpufleet-agent.exe .\cmd\gpufleet-agent
```

前端源码已经构建到 `web/dist` 并可直接由服务端托管。修改前端后重新构建：

```powershell
cd web
npm install
npm run build
cd ..
```

启动服务端：

```powershell
.\bin\gpufleet-server.exe `
  -addr 127.0.0.1:8080 `
  -data-dir data `
  -admin-password change-me `
  -bootstrap-device-id local-dev `
  -bootstrap-secret local-dev-secret `
  -min-free-mb 800
```

启动一次性客户端上报：

```powershell
.\bin\gpufleet-agent.exe `
  -server-url http://127.0.0.1:8080 `
  -device-id local-dev `
  -secret local-dev-secret `
  -once `
  -processes
```

只验证本机采集，不上报：

```powershell
.\bin\gpufleet-agent.exe --print
```

前端浏览器级验证脚本：

```powershell
node scripts\verify-frontend-chrome.mjs --url http://127.0.0.1:8080 --username admin --password change-me --out logs\frontend-verify-manual
```

Web 面板：

```text
http://127.0.0.1:8080
```

默认管理员用户名是 `admin`，密码由 `-admin-password` 指定；若未指定，服务端会在首次启动时生成并打印一次。

## 设备接入

推荐从 Web 面板的“设备”页注册新设备。服务端只在创建和轮换密钥时返回一次性密钥：

```powershell
.\bin\gpufleet-agent.exe `
  -server-url http://your-server:8080 `
  -device-id device_20260602120000 `
  -secret replace-with-one-time-secret
```

禁用设备会让服务端拒绝后续上报；轮换密钥会让旧密钥立即失效。两者都只修改服务端认证记录，不会远程改写客户端配置。

## 推荐生产组件

| 部分 | 选型 | 原因 |
| --- | --- | --- |
| Agent | Go 单文件程序 | 跨平台、部署简单、适合系统服务 |
| 服务端 API | Go 单文件程序 | 依赖少、并发和部署体验好 |
| MVP 时序存储 | gzip JSONL 分段文件 | 零外部依赖、压缩、可按保留期清理 |
| MVP 元数据 | JSON 文件 | 零外部依赖，便于调试 |
| 当前前端 | React + Vite + lucide-react + TanStack Query + ECharts | 多机多卡聚合面板、深浅主题、响应式视图、图标、轮询数据、统计图表 |
| 生产时序数据库 | VictoriaMetrics single-node | 单二进制、内置压缩、保留策略和最小空闲空间保护 |
| 生产元数据数据库 | SQLite | 单文件、零服务依赖、适合用户/设备/密钥/审计数据 |

## 文档

- [产品细节](docs/01-product.md)
- [总体架构](docs/02-architecture.md)
- [安全设计](docs/03-security.md)
- [数据存储与磁盘保护](docs/04-data-storage-retention.md)
- [Agent 设计](docs/05-agent.md)
- [API 契约](docs/06-api-contract.md)
- [Web 前端设计](docs/07-frontend.md)
- [测试与本机验证](docs/08-testing.md)
- [实施路线图](docs/09-roadmap.md)
- [参考资料](docs/10-references.md)

## 默认端口

服务端端口不固定，建议默认使用 `8080` 提供 HTTP API 和 Web 静态资源。

生产环境可以按部署环境选择：

- 直接暴露：`8080`、`8443` 或其他自定义端口。
- 反向代理：Nginx/Caddy/Traefik 监听公网端口，转发到 GPUFleet 服务端。
- HTTPS：生产环境必须启用，可由反向代理或服务端自身承担 TLS。

## 安全边界

GPUFleet 明确不设计以下能力：

- 服务端远程执行客户端命令。
- 服务端修改客户端配置。
- 服务端修改 GPU 功耗、时钟、风扇、MIG、进程等设置。
- 客户端开放公网监听端口。

客户端只读采集，本地配置由设备管理员维护。
