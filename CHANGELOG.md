# Changelog

所有值得用户关注的变更都会记录在这里。项目版本遵循语义化版本思路：`MAJOR.MINOR.PATCH`，当前仍处于 MVP 预览阶段。

User-facing changes are recorded here. Versions follow semantic-versioning ideas (`MAJOR.MINOR.PATCH`), while GPUFleet is still in the MVP preview stage.

## [0.1.7] - 2026-06-05

### Changed / 变更

- Linux 自动更新重启脚本改为先将新二进制原子替换到当前路径，再等待旧进程退出，避免 systemd 在替换前抢先拉起旧二进制。
- Linux update restart helpers now move the new binary into the active path before waiting for the old process to exit, preventing systemd from restarting the old binary first.
- 重启脚本会检测当前二进制路径是否已被其他进程启动，避免 systemd 场景下重复拉起两个服务端进程。
- The restart helper detects whether another process is already running the target binary path to avoid starting a duplicate server under systemd.
- GPU 详情和总览卡片布局进一步压缩，长型号、趋势标题、功耗/显存说明和 Compute 信息不再挤压卡片内容。
- GPU detail and overview card layouts are more compact so long model names, trend labels, power/memory captions, and Compute metadata no longer crowd the card contents.
- 趋势卡片主数值优先保持单行显示，避免功耗、显存等指标在仍有空间时被拆成多行。
- Trend card primary values now prefer single-line display so power, memory, and similar metrics do not wrap while space is still available.
- 趋势卡片主数值改为占满整行，并保护数值与单位不被拆开，避免右侧仍有空间时显示省略号。
- Trend card primary values now span the full row and keep values with their units together, avoiding ellipses while horizontal space remains available.
- GPU 进程列表元信息改为显示设备名称、PID 和 GPU，优先使用设备别名或主机名替代设备 ID。
- GPU process list metadata now shows device name, PID, and GPU, preferring device aliases or hostnames over device IDs.

### Fixed / 修复

- 修复 `0.1.5` 到后续版本自动更新时，Git 仓库已更新但 systemd 仍可能继续运行旧服务端二进制的问题。
- Fixed automatic updates from `0.1.5` and later where the Git checkout updated but systemd could continue running the old server binary.

## [0.1.6] - 2026-06-05

### Added / 新增

- 在线更新状态新增运行版本、运行提交、仓库版本和二进制过期状态，用于区分 Git 工作区提交和当前运行的服务端二进制。
- Online update status now reports running version, running commit, repository version, and binary-outdated state to distinguish the Git checkout from the running server binary.
- 当 Git 仓库已经是最新但运行二进制仍是旧版本时，更新面板会显示“需重建”，并允许执行重建并重启。
- When the Git repository is already current but the running binary is stale, the update panel shows a rebuild-needed state and allows rebuild-and-restart.

### Changed / 变更

- 自动更新构建服务端时写入 `version.Commit` 和 `version.BuildTime`，后续版本检查可准确判断当前运行二进制来源。
- Automatic server builds now stamp `version.Commit` and `version.BuildTime` so later checks can identify the running binary source accurately.
- 无远端新提交但二进制过期时，服务端会从当前仓库提交构建新的二进制并自动重启，不再误判为已经是最新版本。
- When no remote commit is pending but the binary is stale, the server rebuilds from the current repository commit and restarts automatically instead of reporting already up to date.

### Fixed / 修复

- 修复仓库已拉到最新但 `/opt/gpufleet/gpufleet-server` 仍是旧二进制时，在线更新页面显示“最新”且无法触发重建的问题。
- Fixed the update panel showing latest and blocking rebuild when the repository was current but `/opt/gpufleet/gpufleet-server` was still an old binary.

## [0.1.5] - 2026-06-05

### Added / 新增

- 新增在线更新代理地址配置，Git fetch、worktree 构建和 Go build 会复用该代理环境。
- Added an online update proxy setting reused by Git fetch, update worktree builds, and Go build.
- 拉取并重启流程新增明确进度反馈，覆盖请求发送、依赖预检、构建、重启和恢复等待。
- Added explicit progress feedback for pull-and-restart: request, dependency checks, build, restart, and recovery wait.
- 上传 HTTPS 证书后会自动调度服务端重启，恢复后页面自动刷新并弹出提示。
- HTTPS certificate upload now schedules an automatic server restart, refreshes the page after recovery, and shows a notice.
- 版本 API 和设置页 Changelog 新增中英双语字段，英文模式显示英文变更内容。
- Version API and the settings changelog now expose bilingual Chinese and English fields.

### Changed / 变更

- 设置页在线更新状态会缓存 1 小时，打开设置页时优先显示缓存结果，并在后台按小时刷新；点击检查更新会绕过缓存。
- Online update status is cached for one hour, shown immediately on settings open, refreshed hourly in the background, and bypassed by the manual Check update action.
- 首页顶部 KPI 在 overview 尚未加载完成时显示占位符，避免短暂显示 `0/0` 等错误数值。
- Top overview KPI cards show placeholders until overview data is loaded, avoiding transient `0/0` values.
- README 顶部 Logo 调整为小图标加项目名称的横幅形式，并刷新 `imgs` 目录部署截图素材。
- README logo presentation was changed to a compact logo plus project-name banner, and deployment screenshots in `imgs` were refreshed.

### Fixed / 修复

- 修复英文模式下 GPU 卡片相对时间仍显示“前”的问题。
- Fixed GPU card relative time still showing the Chinese suffix in English mode.
- 修复 HTTPS 已启用时仍提示“下次启动生效”的状态文案。
- Fixed HTTPS status copy still saying it would take effect on next start after HTTPS was already active.
- 修复数据库大小为空时设置页只显示短横线的问题。
- Fixed database size showing only a dash when no size had been loaded.
- 修复 HTTPS 证书上传文件选择控件在英文模式下仍显示浏览器原生中文文案的问题。
- Fixed HTTPS certificate file pickers showing browser-native Chinese copy in English mode.

## [0.1.4] - 2026-06-05

### Added

- 设置页数据库下载卡片新增数据库大小显示，对应可下载数据库内容的实际文件大小。

### Changed

- GPU 趋势图悬浮提示从采集点序号改为显示采样时间。
- 内置 fallback 面板同步显示采样时间和数据库大小，保持缺少 `web/dist` 时的体验一致。

## [0.1.3] - 2026-06-05

### Changed

- 服务端在线更新成功并自动重启后，Web 面板会等待服务恢复、自动刷新页面并展示版本更新弹窗。
- 语言保存接口缺失时，前端会提示需要重建并重启服务端，避免只显示 `not found`。
- 服务端启动不再默认创建 `local-dev` 引导设备；只有显式配置 bootstrap device id 和 secret 时才会创建初始设备。

### Fixed

- 修复删除 `local-dev` 后，服务端自动更新或重启又重新创建该设备的问题。

## [0.1.2] - 2026-06-05

### Added

- 新增服务端 i18n 框架，支持首次启动选择简体中文或 English，并将语言配置持久化到服务端元数据。
- 设置页新增语言设置，可在登录后切换中英文并即时生效。
- React 面板新增可扩展词表和动态文案翻译兜底，内置 fallback 面板也补充轻量中英文翻译。
- 文档新增英文 README 和 i18n 维护说明，API、前端、运维和当前实现文档补充语言配置。

### Changed

- 首次配置流程扩展为语言、密码、端口和可选 HTTPS 证书的统一配置流程。
- 服务状态和 overview 响应返回当前语言，便于多浏览器会话保持一致。

## [0.1.1] - 2026-06-05

### Added

- 设备管理支持改名和删除，删除设备后会清理该设备的最新 GPU 状态和最新进程快照。
- 总览新增总功耗指标，并以 GiB 展示全局总显存用量。
- 移动端 GPU 趋势图在小屏继续保持 2x2 布局，并压缩图表高度、字号和提示气泡尺寸。

### Changed

- 设备禁用、启用、删除和密钥轮换统一使用应用内确认弹窗，避免误操作。
- 导航顺序调整为总览、GPU、设备、设置，优先进入多卡监控视角。
- 设置页按“访问与安全”和“维护与发布”分组，改善密码、端口、证书、数据库、在线更新、配置引导和版本信息的排版稳定性。
- 服务端在线更新从单纯 fast-forward 拉取升级为依赖预检、远端提交构建、fast-forward 拉取和 Windows/Linux 自动重启。

### Security

- 高风险设备操作需要二次确认，降低误禁用、误删除和误轮换密钥风险。
- 在线更新拒绝前端传入命令、路径、分支和远端；缺少 `git`、`go` 或平台重启器依赖时会在拉取前阻止更新。

### Fixed

- 修复设置页视觉混排，以及旧式指标文案导致的信息不清晰问题。

## [0.1.0] - 2026-06-03

### Added

- 支持 Windows 和 Linux NVIDIA GPU 设备的客户端-服务端架构。
- React Web 面板提供多设备 GPU 卡片、2x2 历史图表、深浅主题、移动端底部导航和 SVG 品牌 Logo。
- 首次启动交互式配置访问密码、访问端口和可选 HTTPS 证书。
- 登录后的设置页可查看版本号、构建信息和最近变更。
- 设置页可检查服务端 Git upstream，并在工作区干净且可 fast-forward 时拉取更新。
- Agent 支持只读采集 GPU 运行字段、GPU 进程快照和本地离线队列。
- 服务端支持 gzip JSONL 分段压缩存储、保留期清理、数据库下载和 800MiB 默认磁盘预留保护。

### Changed

- 设置页聚合服务状态、密码更改、端口配置、HTTPS 证书、数据库下载、配置引导、项目署名和发布信息。
- 缺少 `web/dist` 时，内置 fallback 面板仍保留主要展示、在线更新和运维入口。

### Security

- Agent 上报使用 HMAC-SHA256 签名，带时间戳、nonce 和重放保护。
- Web 登录仅使用密码凭据，并记住当前浏览器设备 30 天。
- 登录入口具备短窗口限流和递进锁定的防爆破保护。
- 服务端保持只接收数据，不提供客户端命令下发、配置下发或远程执行 API。
- 在线更新接口只执行固定 Git 参数，拒绝 dirty、无 upstream、本地超前或分叉工作区。
