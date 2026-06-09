# Changelog

所有值得用户关注的变更都会记录在这里。项目版本遵循语义化版本思路：`MAJOR.MINOR.PATCH`，当前仍处于 MVP 预览阶段。

User-facing changes are recorded here. Versions follow semantic-versioning ideas (`MAJOR.MINOR.PATCH`), while GPUFleet is still in the MVP preview stage.

## [0.1.8] - 2026-06-09

### Title / 标题

- zh-CN: 数据洞察与监控呈现增强
- en-US: Data insight and monitoring presentation improvements

### Changed / 变更

- zh-CN: 总览高温 GPU 统计统一使用服务端 85°C 高温口径，避免和卡片健康状态出现不一致。
- en-US: Overview hot-GPU counts now use the same server-side 85°C threshold as card health state to avoid inconsistent totals.
- zh-CN: 利用率分布图按利用率排序，并在横轴和悬浮提示中显示设备名称与 GPU ID，避免多设备 `gpu0` 标签混淆。
- en-US: Utilization distribution charts are sorted by utilization and label each bar with the device name plus GPU ID, avoiding ambiguous multi-device `gpu0` labels.
- zh-CN: 总览巡检摘要新增 PCIe 降级和时钟限速 GPU 计数，并在 GPU 卡片元信息中突出这些需要关注的硬件状态。
- en-US: Overview inspection facts now include PCIe-degraded and clock-throttled GPU counts, and GPU cards highlight those hardware states in their metadata.
- zh-CN: 统计面板新增 1H、6H、24H、7D 和 30D 时间范围切换，展开曲线会按所选范围加载。
- en-US: Stats panels now support 1H, 6H, 24H, 7D, and 30D range switching, and expanded charts load the selected range.
- zh-CN: 统计面板新增筛选、排序和摘要卡片，可按平均/峰值利用率、空闲率、峰值显存、峰值温度、峰值功耗和样本数分析 GPU。
- en-US: Stats panels now include filters, sorting, and summary cards for analyzing GPUs by average/peak utilization, idle rate, peak memory, peak temperature, peak power, and sample count.
- zh-CN: 前端补齐服务端已返回的统计字段，显示采样覆盖范围、平均显存和峰值利用率，减少只看瞬时快照造成的误判。
- en-US: The frontend now consumes the richer stats fields already returned by the server, showing sample coverage, average memory, and peak utilization to reduce snapshot-only misreads.
- zh-CN: 版本号、README、前端包元数据和内置版本 API 变更记录同步到 0.1.8。
- en-US: Version numbers, README files, frontend package metadata, and the built-in version API changelog fallback now point to 0.1.8.

### Fixed / 修复

- zh-CN: 修复 GPU 卡片 tag 区域在 PCIe 降级文案较长时出现横向滚动条的问题，标签改为固定网格并保留完整 hover 提示。
- en-US: Fixed GPU card tag rows showing a horizontal scrollbar when PCIe degradation labels were long; tags now use a fixed grid while preserving the full hover tooltip.
- zh-CN: 修复统计面板展开长时间范围曲线时可能触发过多 series 请求、显示空图或 `Failed to fetch` 的问题；长范围曲线改用聚合索引，并优化统计行与控制区的窄宽度排版。
- en-US: Fixed stats panels potentially firing too many series requests and showing empty charts or `Failed to fetch` for long ranges; long-range charts now use rollup indexes, with tighter responsive layout for stat rows and controls.
- zh-CN: 优化 GPU 卡片 tag 胶囊高度和移动端三列排布，长文本改为自动跑马条显示，不再直接截断为省略号。
- en-US: Refined GPU card tag pill height and kept three columns on mobile; long tag text now auto-scrolls marquee-style instead of truncating directly to an ellipsis.
- zh-CN: 修复更新后等待服务端恢复时反复执行 Git fresh 检查导致等待时间明显变长的问题，恢复检测改为轻量版本确认。
- en-US: Fixed post-update recovery waiting taking much longer because it repeatedly ran fresh Git checks; recovery detection now uses a lightweight version check.
- zh-CN: 修复 6H 以上统计曲线在启动初期或 30D 边界附近可能回退扫描原始指标并加载失败的问题，长范围曲线增加 rollup 边界容错和前端短重试。
- en-US: Fixed 6H+ stats charts potentially falling back to raw metric scans and failing during startup or near the 30D boundary; long-range charts now add rollup boundary tolerance and short frontend retries.
- zh-CN: GPU 卡片限速 tag 现在同时显示当前 P-state；移动端曲线点位提示改为触摸后短暂停留，便于查看具体数值。
- en-US: GPU card throttle tags now include the current P-state, and mobile chart point tooltips stay visible briefly after touch for easier value inspection.
- zh-CN: 修复顶部汇总小曲线和趋势图 tooltip 可能被相邻卡片遮挡的问题，并统一顶部平均利用率、总显存用量和总功耗 tooltip 与下方图表的尺寸样式。
- en-US: Fixed top summary and trend-chart tooltips potentially being covered by neighboring cards, and aligned the top average-utilization, memory, and power tooltip sizing with the lower charts.
- zh-CN: 修复更新恢复和自动更新监控对短 commit 与完整 commit 严格相等匹配导致的等待重启不结束、二进制落后误判和反复重启问题；自动监控会跳过刚完成的同目标重建，安装脚本改为注入完整 commit。
- en-US: Fixed update recovery and automatic-update monitoring treating short and full commit hashes as different, which could keep restart waiting active, misreport the binary as stale, and trigger repeated restarts; automatic monitoring now skips just-completed same-target rebuilds, and the Linux installer stamps full commits.

## [0.1.7] - 2026-06-08

### Title / 标题

- zh-CN: 安装、自动更新、GPU 监控、设置与存储优化
- en-US: Installation, automatic updates, GPU monitoring, settings, and storage improvements

### Added / 新增

- zh-CN: 新增默认开启的服务端自动更新检查，每 30 分钟检测 Git 上游，有可 fast-forward 更新时自动拉取、构建并调度重启。
- en-US: Added default-on server-side automatic update checks every 30 minutes; fast-forwardable upstream updates are pulled, built, and scheduled for restart automatically.
- zh-CN: 自动更新完成后，下一次管理员访问会弹出更新提示，展示更新时间和更新内容；同版本更新会只显示新增或变化的 CHANGELOG 行，完全一致时显示“无更新说明”。
- en-US: After an automatic update completes, the next admin visit shows an update notice with the update time and notes; same-version updates show only new or changed CHANGELOG lines, or "No update notes" when unchanged.

### Changed / 变更

- zh-CN: 移动端配置引导改为更紧凑的首屏摘要和表单布局，窄屏下减少英雄区占用并保持保存操作易触达。
- en-US: Mobile setup now uses a more compact first-screen summary and form layout, reducing hero height on narrow screens while keeping save actions easy to reach.
- zh-CN: Linux 自动更新重启脚本改为先将新二进制原子替换到当前路径，再等待旧进程退出，避免 systemd 在替换前抢先拉起旧二进制。
- en-US: Linux update restart helpers now move the new binary into the active path before waiting for the old process to exit, preventing systemd from restarting the old binary first.
- zh-CN: 重启脚本会检测当前二进制路径是否已被其他进程启动，避免 systemd 场景下重复拉起两个服务端进程。
- en-US: The restart helper detects whether another process is already running the target binary path to avoid starting a duplicate server under systemd.
- zh-CN: GPU 详情和总览卡片布局进一步压缩，长型号、趋势标题、功耗/显存说明和 Compute 信息不再挤压卡片内容。
- en-US: GPU detail and overview card layouts are more compact so long model names, trend labels, power/memory captions, and Compute metadata no longer crowd the card contents.
- zh-CN: 趋势卡片主数值优先保持单行显示并占满整行，避免功耗、显存等指标在仍有空间时被拆成多行或显示省略号。
- en-US: Trend card primary values now prefer single-line display and span the full row, avoiding wraps or ellipses while horizontal space remains available.
- zh-CN: GPU 进程和 24 小时统计列表元信息优先显示设备名称，减少直接展示设备 ID。
- en-US: GPU process and 24-hour stats metadata now prefer device names, reducing direct device ID display.
- zh-CN: 总览和 GPU 监控页的汇总指标卡新增右侧迷你曲线，用于展示各 GPU 当前利用率、显存和功耗分布。
- en-US: Overview and GPU monitoring aggregate metric cards now include right-side sparklines for current per-GPU utilization, memory, and power distribution.
- zh-CN: 设置页数据库下载卡片改为显示实际已存储天数，并将 7 天外指标分段重压缩为单个高压缩率 gzip 成员以降低长期存储占用。
- en-US: The settings database download card now shows actual stored days, and metric segments older than 7 days are recompressed into single high-compression gzip members to reduce long-term storage use.
- zh-CN: 在线更新检查失败时会按 GitHub TLS、DNS、连接超时和认证等常见原因显示可操作提示，并保留 Git 原始错误供详情弹窗诊断。
- en-US: Online update check failures now show actionable messages for common GitHub TLS, DNS, timeout, and authentication issues while preserving raw Git errors in a details dialog for diagnosis.
- zh-CN: 自动更新与普通更新检测统一为同一套后台监测逻辑：启动时立即检查，关闭自动更新时每 1 小时检查并在设置入口提示，开启自动更新时每 30 分钟检查并可立即自动更新。
- en-US: Automatic updates and regular update checks now share one background monitor: startup checks immediately, disabled auto-update checks hourly and flags Settings, and enabled auto-update checks every 30 minutes with immediate automatic application when available.
- zh-CN: 移动端配置引导顶部加入浏览器安全区间距，并改用动态视口高度，避免窄屏浏览器顶部内容被裁切。
- en-US: Mobile setup now adds browser safe-area spacing and dynamic viewport height so the top of the wizard is not clipped in narrow mobile browsers.
- zh-CN: 指标趋势和统计查询改为分段级读写锁，读取 gzip 分段时不再持有全局指标锁，减少多卡趋势查询对写入上报的阻塞。
- en-US: Metric trend and stats queries now use per-segment read/write locks, so gzip segment scans no longer hold the global metrics lock and multi-GPU trend reads block writes less.
- zh-CN: 指标存储新增内存索引和 rollup：最近 1 小时趋势使用原始点索引，24 小时统计优先使用分钟级汇总，7/30 天窗口可使用小时级汇总，降低前端多卡统计反复扫描压缩分段的压力。
- en-US: Metrics now maintain in-memory indexes and rollups: recent 1-hour trends use raw point indexes, 24-hour stats prefer minute rollups, and 7/30-day windows can use hourly rollups to reduce repeated compressed-segment scans.
- zh-CN: `metadata.json` 增加 `schema_version` 并在启动时统一迁移旧字段默认值，后续元数据演进不再完全依赖零值兼容。
- en-US: `metadata.json` now includes `schema_version` and startup migrations for legacy defaults, so future metadata changes no longer rely solely on zero-value compatibility.
- zh-CN: 在线更新替换服务端二进制前会保留上一版 `.bak`，重启脚本在替换或启动阶段失败时会尽量恢复旧二进制。
- en-US: Online updates now keep a `.bak` copy of the previous server binary before replacement, and restart helpers try to restore it if replacement or startup fails.
- zh-CN: 关键 JSON 和证书文件写入改为临时文件、文件 flush、rename，并尽量同步目录，提升异常断电或进程中断时的数据文件可靠性。
- en-US: Critical JSON and certificate writes now use temporary files, file flush, rename, and best-effort directory sync for better resilience against power loss or process interruption.
- zh-CN: 在线更新进度改为背景模糊加前景进度面板展示，并加入百分比、进度条和阶段动画以提升更新体验。
- en-US: Online update progress now uses a blurred backdrop with a foreground progress panel, percentage, progress bar, and staged animation for a clearer update experience.
- zh-CN: 24 小时统计列表支持点击 GPU 展开过去 24H 的利用率、显存、温度和功耗曲线，GPU 监控页统计面板宽度与详情卡片主列对齐。
- en-US: 24-hour stats rows can now expand per GPU to show 24-hour utilization, memory, temperature, and power charts, with the GPU monitoring stats panel aligned to the detail-card column width.
- zh-CN: 版本 API 和设置页 Changelog 改为优先读取仓库中的 CHANGELOG.md，并规范化中英文条目格式。
- en-US: Version API and settings changelog now prefer reading CHANGELOG.md from the repository, with normalized bilingual entry formatting.
- zh-CN: 新增 Linux 服务端一键安装脚本，支持 clone 后构建、写入 systemd 服务、开机自启动并使用当前仓库作为在线更新来源。
- en-US: Added a one-command Linux server installer that builds after clone, writes a systemd service, enables boot startup, and uses the current checkout as the online-update source.
- zh-CN: 新增双语安装指南，覆盖 Linux 服务端、旧部署升级、Linux/Windows/WSL2 设备端 Agent、服务命令、卸载和连通性检查。
- en-US: Added a bilingual installation guide covering Linux server setup, older deployment upgrades, Linux/Windows/WSL2 Agents, service commands, uninstall steps, and connectivity checks.
- zh-CN: 优化移动端顶部 GPU 指标卡布局，带迷你曲线的总显存用量和总功耗卡片不再挤压文字，GPU 页顶部卡片在小屏保持更紧凑的双列展示。
- en-US: Improved mobile GPU metric card layouts so memory and power sparklines no longer squeeze text, while GPU-page top cards stay in a more compact two-column layout on small screens.
- zh-CN: GPU 详细状态的参数网格在移动端保持两列紧凑展示，减少单张 GPU 卡片的纵向占用。
- en-US: GPU detail cards now keep their parameter grid in a compact two-column layout on mobile, reducing vertical space per GPU card.
- zh-CN: 在线更新卡片移除超前、运行提交和远端字段，更新按钮改为二次确认后执行，并新增可立即生效的磁盘预留空间设置。
- en-US: The online update card now removes ahead, running commit, and remote fields, requires confirmation before updating, and adds an immediately applied disk reserve setting.
- zh-CN: 首次配置和设置页重新打开配置引导统一为同一套全屏品牌化向导，重新打开时会预填此前端口、语言和证书状态。
- en-US: First-time setup and the settings-page setup wizard now share the same full-screen branded flow, with reopened setup prefilled from the existing port, language, and certificate state.
- zh-CN: 设置页新增手动重启服务操作，确认后以全屏进度等待服务恢复，并在恢复刷新后弹出重启成功提示。
- en-US: Added a manual service restart action in settings, with a full-screen recovery progress view and a restart success notice after refresh.
- zh-CN: 新增访客功能，可在设置页开启登录页访客入口；访客页仅展示脱敏 GPU 总览，不开放进程、24 小时统计或管理接口，并在设置页提供含浏览器指纹摘要的访客记录弹窗。
- en-US: Added guest access with a settings-controlled login entry; the guest page only shows a sanitized GPU overview without processes, 24-hour stats, or admin APIs, and settings now include guest visit records with browser fingerprint summaries.
- zh-CN: 设置页更新记录默认只展示当前版本，更多更新记录改为弹窗查看完整 CHANGELOG；访客记录弹窗改为固定头部和内部滚动列表，长记录不会撑出屏幕。
- en-US: Settings now shows only the current version by default, opens the full CHANGELOG in a dialog from the More changelog button, and keeps guest visit records scrollable inside the dialog without overflowing the viewport.
- zh-CN: 全面统一设置页、设备操作、更新提示、更新进度、重启、访客记录和内置 fallback 面板的弹窗遮罩，确保弹窗始终挂载到全屏视口并使用一致的背景模糊。
- en-US: Unified dialog backdrops for settings, device actions, update notices, update progress, restarts, guest records, and the built-in fallback panel so dialogs always cover the full viewport with consistent background blur.

### Security / 安全

- zh-CN: 管理员密码派生改为 PBKDF2-SHA256，旧版自定义 SHA-256 多轮 hash 会在登录成功后自动迁移。
- en-US: Admin password derivation now uses PBKDF2-SHA256, and legacy custom multi-round SHA-256 hashes are migrated after a successful login.
- zh-CN: Agent 上报改为先校验时间戳和 HMAC 签名，再原子记录 nonce，避免无效请求污染 nonce 集合。
- en-US: Agent reports now verify timestamp and HMAC signatures before atomically recording nonces, preventing invalid requests from polluting the nonce set.
- zh-CN: 默认 CSP 移除脚本侧 `unsafe-inline`；仅在缺少 web/dist、使用内置 fallback 面板时保留内联脚本兼容策略。
- en-US: The default CSP now removes script-side `unsafe-inline`; inline script compatibility is kept only for the built-in fallback panel when web/dist is unavailable.
- zh-CN: 已登录管理写接口新增 Origin/Referer 同源校验，重启、在线更新、证书上传、设备删除等高风险 POST/PATCH/DELETE 请求不再只依赖 SameSite Cookie。
- en-US: Authenticated management write APIs now validate same-origin Origin/Referer headers, so high-risk POST/PATCH/DELETE actions such as restart, online update, certificate upload, and device deletion no longer rely only on SameSite cookies.
- zh-CN: 审计日志扩展 actor、remote_ip、device_id 和 request_id 字段，高风险管理操作会额外记录结构化上下文。
- en-US: Audit logs now include actor, remote_ip, device_id, and request_id fields, and high-risk management actions record additional structured context.

### Fixed / 修复

- zh-CN: 修复 0.1.5 到后续版本自动更新时，Git 仓库已更新但 systemd 仍可能继续运行旧服务端二进制的问题。
- en-US: Fixed automatic updates from 0.1.5 and later where the Git checkout updated but systemd could continue running the old server binary.
- zh-CN: 修复 GPU 监控页离线 GPU 卡片没有遮罩的问题，离线遮罩现在会同时显示离线时长。
- en-US: Fixed missing offline masks on GPU monitoring cards; the mask now also shows how long the GPU has been offline.
- zh-CN: 修复离线 GPU 重新上线后，总览和 GPU 监控页小图表可能继续沿用空曲线缓存的问题，并统一 GPU 详情数值悬浮提示样式。
- en-US: Fixed overview and GPU monitoring sparklines potentially reusing empty series caches after an offline GPU comes back online, and aligned GPU detail value hover tooltips with the chart tooltip style.
- zh-CN: 修复 GPU 最新快照变化时小图表反复重建刷新的问题，并将当前快照补入曲线末端，让总览和 GPU 监控页的总功耗曲线与当前数值保持一致。
- en-US: Fixed sparklines repeatedly rebuilding as GPU snapshots changed, and appends the current snapshot to chart tails so overview and GPU monitoring power trends match the current value.
- zh-CN: 调小总览和 GPU 监控页顶部迷你图表悬浮提示的数值字号，避免提示层遮挡时过于突兀。
- en-US: Reduced the value font size in top metric sparkline hover tooltips on Overview and GPU monitoring so the overlay feels less intrusive.
- zh-CN: 修复英文界面下数据库下载、磁盘预留和部分更新提示仍可能显示中文的问题，切回中文时也会立即恢复中文文案。
- en-US: Fixed Database Download, Disk Reserve, and some update messages still showing Chinese in English mode, and made switching back to Chinese apply immediately.
- zh-CN: 访客页面语言改为跟随访客浏览器语言，不再沿用管理员保存的界面语言。
- en-US: Guest pages now follow the visitor browser language instead of inheriting the admin-saved interface language.
- zh-CN: 修复自动更新完成提示始终显示“无更新说明”的问题，现在会在拉取前比较旧提交和目标提交的 changelog 差异。
- en-US: Fixed automatic update completion notices always showing "No update notes"; changelog differences are now compared before the pull using the old and target commits.
- zh-CN: 修复手动在线更新重启后只显示版本更新、不显示变更内容的问题，手动更新现在也会复用服务端 changelog 差异摘要。
- en-US: Fixed manual online updates only showing a version-updated dialog after restart; manual updates now reuse the server-side changelog diff summary too.
- zh-CN: 修复系统更新重启后仍沿用浏览器旧更新状态缓存，导致设置入口继续提示有新版本的问题；更新恢复后会立即刷新并写入最新检查状态。
- en-US: Fixed stale browser update-status caches after a system update restart that kept Settings flagged as having an update; recovery now refreshes and stores the latest check immediately.
- zh-CN: 修复仅重建落后服务端二进制时更新说明仍显示“无更新说明”的问题，现在会按运行中的 commit 到目标 commit 计算 changelog 差异，并在前端保留更新响应里的说明作为重启回退。
- en-US: Fixed rebuild-only updates for stale server binaries still showing "No update notes"; changelog diffs now compare the running commit to the target commit, and the frontend keeps response notes as a restart fallback.
- zh-CN: 修复访客脱敏设备 ID 由 map 顺序生成导致访客 GPU 曲线偶发查不到真实设备的问题。
- en-US: Fixed guest GPU series occasionally resolving to the wrong real device because sanitized guest device IDs were generated from map iteration order.
- zh-CN: 修复在线更新失败提示里的 Git 原始错误问号按钮被推到卡片右侧或单独换行的问题，现在会紧跟提示文本末尾显示。
- en-US: Fixed the raw Git error help button in online update failure messages being pushed to the card edge or onto its own line; it now stays inline after the message text.

## [0.1.6] - 2026-06-05

### Title / 标题

- zh-CN: 自动更新二进制一致性检测
- en-US: Automatic update binary consistency detection

### Added / 新增

- zh-CN: 在线更新状态新增运行版本、运行提交、仓库版本和二进制过期状态，用于区分 Git 工作区提交和当前运行的服务端二进制。
- en-US: Online update status now reports running version, running commit, repository version, and binary-outdated state to distinguish the Git checkout from the running server binary.
- zh-CN: 当 Git 仓库已经是最新但运行二进制仍是旧版本时，更新面板会显示“需重建”，并允许执行重建并重启。
- en-US: When the Git repository is already current but the running binary is stale, the update panel shows a rebuild-needed state and allows rebuild-and-restart.

### Changed / 变更

- zh-CN: 自动更新构建服务端时写入 version.Commit 和 version.BuildTime，后续版本检查可准确判断当前运行二进制来源。
- en-US: Automatic server builds now stamp version.Commit and version.BuildTime so later checks can identify the running binary source accurately.
- zh-CN: 无远端新提交但二进制过期时，服务端会从当前仓库提交构建新的二进制并自动重启，不再误判为已经是最新版本。
- en-US: When no remote commit is pending but the binary is stale, the server rebuilds from the current repository commit and restarts automatically instead of reporting already up to date.

### Fixed / 修复

- zh-CN: 修复仓库已拉到最新但 /opt/gpufleet/gpufleet-server 仍是旧二进制时，在线更新页面显示“最新”且无法触发重建的问题。
- en-US: Fixed the update panel showing latest and blocking rebuild when the repository was current but /opt/gpufleet/gpufleet-server was still an old binary.

## [0.1.5] - 2026-06-05

### Title / 标题

- zh-CN: 在线更新体验、HTTPS 证书重启与双语变更记录
- en-US: Online update UX, HTTPS certificate restart, and bilingual changelog

### Added / 新增

- zh-CN: 在线更新新增代理地址配置，Git fetch、worktree 构建和 Go build 会复用该代理环境。
- en-US: Added an online update proxy setting reused by Git fetch, update worktree builds, and Go build.
- zh-CN: 拉取并重启流程新增明确进度反馈，覆盖请求发送、依赖预检、构建、重启和恢复等待。
- en-US: Added explicit progress feedback for pull-and-restart: request, dependency checks, build, restart, and recovery wait.
- zh-CN: 上传 HTTPS 证书后会自动调度服务端重启，恢复后页面自动刷新并弹出提示。
- en-US: HTTPS certificate upload now schedules an automatic server restart, refreshes the page after recovery, and shows a notice.
- zh-CN: 版本 API 和设置页 Changelog 新增中英双语字段，英文模式显示英文变更内容。
- en-US: Version API and the settings changelog now expose bilingual Chinese and English fields.

### Changed / 变更

- zh-CN: 设置页在线更新状态会缓存 1 小时，打开设置页时优先显示缓存结果，并在后台按小时刷新。
- en-US: Online update status is cached for one hour, shown immediately on settings open, and refreshed hourly in the background.
- zh-CN: 点击检查更新会绕过缓存并立即重新检查 Git upstream 状态。
- en-US: The Check update button bypasses the cache and rechecks Git upstream immediately.
- zh-CN: 首页顶部 KPI 在 overview 尚未加载完成时显示占位符，避免短暂显示 0/0 等错误数值。
- en-US: Top overview KPI cards show placeholders until overview data is loaded, avoiding transient 0/0 values.
- zh-CN: README 顶部 Logo 调整为小图标加项目名称的横幅形式，避免首屏被大图占满。
- en-US: README logo presentation was changed to a compact logo plus project-name banner.
- zh-CN: 刷新 imgs 目录中的部署截图素材。
- en-US: Refreshed deployment screenshots in the imgs directory.

### Fixed / 修复

- zh-CN: 修复英文模式下 GPU 卡片相对时间仍显示“前”的问题。
- en-US: Fixed GPU card relative time still showing the Chinese suffix in English mode.
- zh-CN: 修复 HTTPS 已启用时仍提示“下次启动生效”的状态文案。
- en-US: Fixed HTTPS status copy still saying it would take effect on next start after HTTPS was already active.
- zh-CN: 修复数据库大小为空时设置页只显示短横线的问题。
- en-US: Fixed database size showing only a dash when no size had been loaded.
- zh-CN: 修复 HTTPS 证书上传文件选择控件在英文模式下仍显示浏览器原生中文文案的问题。
- en-US: Fixed HTTPS certificate file pickers showing browser-native Chinese copy in English mode.

## [0.1.4] - 2026-06-05

### Title / 标题

- zh-CN: 图表悬浮时间与数据库大小

### Added / 新增

- zh-CN: 设置页数据库下载卡片新增数据库大小显示，对应可下载数据库内容的实际文件大小。

### Changed / 变更

- zh-CN: GPU 趋势图悬浮提示从采集点序号改为显示采样时间。
- zh-CN: 内置 fallback 面板同步显示采样时间和数据库大小，保持缺少 web/dist 时的体验一致。

## [0.1.3] - 2026-06-05

### Title / 标题

- zh-CN: 更新重启反馈与默认设备修复

### Changed / 变更

- zh-CN: 服务端在线更新成功并自动重启后，Web 面板会等待服务恢复、自动刷新页面并展示版本更新弹窗。
- zh-CN: 语言保存接口缺失时，前端会提示需要重建并重启服务端，避免只显示 not found。
- zh-CN: 服务端启动不再默认创建 local-dev 引导设备；只有显式配置 bootstrap device id 和 secret 时才会创建初始设备。

### Fixed / 修复

- zh-CN: 修复删除 local-dev 后，服务端自动更新或重启又重新创建该设备的问题。

## [0.1.2] - 2026-06-05

### Title / 标题

- zh-CN: 服务端国际化框架

### Added / 新增

- zh-CN: 新增服务端语言配置，支持首次配置时选择简体中文或 English，并持久化到 metadata.json。
- zh-CN: 新增设置页语言切换能力，语言偏好会同步到服务端并立即影响 Web 面板。
- zh-CN: 新增可扩展前端 i18n 词表和动态文案翻译兜底，覆盖 React 面板和内置 fallback 面板的主要用户可见文案。
- zh-CN: 新增英文 README 和 i18n 维护文档，API、前端、运维和当前实现文档补充语言配置说明。

### Changed / 变更

- zh-CN: 首次配置流程扩展为密码、端口、语言和可选 HTTPS 证书的统一配置流程。
- zh-CN: 服务状态 API、overview API 和设置相关响应现在返回当前语言字段，便于多浏览器保持一致界面语言。

### Fixed / 修复

- zh-CN: 补齐设置页、更新页、设备管理、指标卡片和错误提示等界面的中英文文案维护入口，降低后续新增语言时遗漏文案的风险。

## [0.1.1] - 2026-06-05

### Title / 标题

- zh-CN: 设备管理与移动端体验增强

### Added / 新增

- zh-CN: 设备管理支持改名和删除，删除后会清理该设备的最新 GPU 与进程快照。
- zh-CN: 总览新增总功耗指标，并以 GiB 展示全局总显存用量。
- zh-CN: 移动端 GPU 趋势图在小屏继续保持 2x2 布局，并压缩图表尺寸以减少滚动。

### Changed / 变更

- zh-CN: 设备页中的禁用、启用、删除和密钥轮换统一使用应用内确认弹窗。
- zh-CN: 导航顺序调整为总览、GPU、设备、设置，优先进入多卡监控视角。
- zh-CN: 设置页按访问与安全、维护与发布重新分组，保留密码、端口、证书、数据库、在线更新、配置引导和版本信息。
- zh-CN: 服务端在线更新从单纯 fast-forward 拉取升级为依赖预检、远端提交构建、fast-forward 拉取和 Windows/Linux 自动重启。

### Security / 安全

- zh-CN: 高风险设备操作需要二次确认，降低误禁用、误删除和误轮换密钥风险。
- zh-CN: 在线更新拒绝前端传入命令、路径、分支和远端；缺少 git、go 或平台重启器依赖时会在拉取前阻止更新。

### Fixed / 修复

- zh-CN: 修复服务设置页视觉混排和旧式指标文案导致的信息不清晰问题。

## [0.1.0] - 2026-06-03

### Title / 标题

- zh-CN: MVP 预览版：安全的多设备 GPU 可观测面板

### Added / 新增

- zh-CN: 支持 Windows 和 Linux NVIDIA GPU 设备的客户端-服务端架构。
- zh-CN: React 面板提供多设备 GPU 卡片、历史图表、深浅主题、移动端底部导航和 SVG 品牌 Logo。
- zh-CN: 首次启动交互式配置访问密码、端口和可选 HTTPS 证书。
- zh-CN: 登录后的设置页可查看版本号、构建信息和变更记录。
- zh-CN: 设置页可检查服务端 Git upstream，并在工作区干净且可 fast-forward 时拉取更新。

### Changed / 变更

- zh-CN: 设置页聚合项目署名、数据库下载、证书状态和发布信息。
- zh-CN: 缺少 web/dist 时，内置 fallback 面板仍保留主要展示、在线更新和运维入口。

### Security / 安全

- zh-CN: Agent 上报使用 HMAC 签名并带 nonce 重放保护。
- zh-CN: Web 登录仅使用密码凭据，并记住当前浏览器设备 30 天。
- zh-CN: 登录入口具备短窗口限流和递进锁定的防爆破保护。
- zh-CN: 服务端保持只接收数据，不暴露客户端命令或配置下发接口。
- zh-CN: 在线更新接口只执行固定 Git 参数，拒绝 dirty、无 upstream、本地超前或分叉工作区。
