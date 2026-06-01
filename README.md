# GPUFleet

GPUFleet 是一个用于聚合多台 Windows/Linux 设备上 NVIDIA GPU 运行信息的客户端-服务端系统。

核心目标：

- 客户端 Agent 只做本机只读采集，主动向公网服务端上报。
- 服务端只接收、存储、统计和展示数据，不向客户端下发命令。
- 依赖保持简单，MVP 不引入 Kubernetes、Kafka、Prometheus、Grafana。
- 时序数据具备压缩、保留策略和磁盘保护能力，默认预留 800MiB 空闲空间。
- Web 面板适配桌面、平板和移动端，使用现代组件库构建。

## 推荐 MVP 组件

| 部分 | 选型 | 原因 |
| --- | --- | --- |
| Agent | Go 单文件程序 | 跨平台、部署简单、适合系统服务 |
| 服务端 API | Go 单文件程序 | 依赖少、并发和部署体验好 |
| 时序数据库 | VictoriaMetrics single-node | 单二进制、内置压缩、保留策略和最小空闲空间保护 |
| 元数据数据库 | SQLite | 单文件、零服务依赖、适合用户/设备/密钥/审计数据 |
| 前端 | React + Vite + TypeScript | 成熟、生态好、构建简单 |
| UI | Tailwind CSS + shadcn/ui + lucide-react | 现代、响应式、组件可控 |
| 图表 | Apache ECharts | GPU 指标图表能力强，移动端表现好 |

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

