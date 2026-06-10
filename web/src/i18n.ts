import { createContext, useContext } from 'react';
import { AppLanguage } from './api';

export type Translate = (key: string, values?: Record<string, string | number>) => string;

export const languages: Array<{ code: AppLanguage; label: string; nativeLabel: string }> = [
  { code: 'zh-CN', label: 'Chinese', nativeLabel: '简体中文' },
  { code: 'en-US', label: 'English', nativeLabel: 'English' }
];

const en: Record<string, string> = {
  '正在连接': 'Connecting',
  '检查当前 Web 会话': 'Checking the current web session',
  '未选择': 'Not selected',
  '端口范围应为 1-65535': 'Port must be between 1 and 65535',
  '密码至少 8 位': 'Password must be at least 8 characters',
  '新密码至少 8 位': 'New password must be at least 8 characters',
  '两次密码不一致': 'Passwords do not match',
  '证书和私钥需要同时上传': 'Certificate and private key must be uploaded together',
  '配置已保存，重启服务后端口或 HTTPS 生效': 'Configuration saved. Restart the service for port or HTTPS changes to take effect.',
  '配置已保存，必要时重启服务后生效': 'Configuration saved. Restart the service if required.',
  '配置已保存': 'Configuration saved',
  '首次配置': 'First-time setup',
  '配置引导': 'Setup wizard',
  '初始化服务访问参数': 'Initialize service access settings',
  '访问密码': 'Access password',
  '新密码': 'New password',
  '至少 8 位': 'At least 8 characters',
  '留空则不变': 'Leave blank to keep unchanged',
  '确认密码': 'Confirm password',
  '再次输入密码': 'Enter the password again',
  '仅修改密码时填写': 'Required only when changing the password',
  '访问端口': 'Access port',
  '界面语言': 'Interface language',
  'HTTPS 证书': 'HTTPS certificate',
  '私钥文件': 'Private key file',
  '取消': 'Cancel',
  '保存中': 'Saving',
  '保存配置': 'Save configuration',
  '设置服务端访问方式后即可进入控制台': 'Set the server access options, then enter the console.',
  '此前配置已预填，可只修改需要变更的项目': 'Previous settings are prefilled. Change only what needs updating.',
  '可选': 'Optional',
  '当前证书到期：{date}': 'Current certificate expires: {date}',
  '登录面板': 'Dashboard login',
  '登录后记住当前设备 30 天': 'This device is remembered for 30 days after login',
  '密码': 'Password',
  '登录中': 'Logging in',
  '登录': 'Log in',
  'GPU 资源总览': 'GPU Resource Overview',
  '设备管理': 'Device Management',
  'GPU 监控': 'GPU Monitoring',
  '热能与能源': 'Thermal & Energy',
  '服务设置': 'Service Settings',
  '总览': 'Overview',
  '设备': 'Devices',
  '能耗': 'Energy',
  '设置': 'Settings',
  '服务端时间 {time}': 'Server time {time}',
  '等待服务端数据': 'Waiting for server data',
  '刷新': 'Refresh',
  '退出登录': 'Log out',
  '磁盘空间低于保护阈值，服务端已拒绝新指标写入。': 'Disk space is below the protection threshold. New metric writes are rejected.',
  '切换浅色': 'Switch to light mode',
  '切换深色': 'Switch to dark mode',
  '多机 GPU 运行态': 'Multi-host GPU Runtime',
  '{devices} 台设备，{gpus} 块 GPU，按最新上报状态汇总。': '{devices} devices, {gpus} GPUs, summarized from the latest reports.',
  '等待客户端上报 GPU 运行信息。': 'Waiting for clients to report GPU runtime data.',
  '在线设备': 'Online devices',
  'GPU 总数': 'Total GPUs',
  '{count} 块 GPU': '{count} GPUs',
  '{count} 个样本': '{count} samples',
  '{count} 项': '{count} items',
  '忙碌 GPU': 'Busy GPUs',
  '高温 GPU': 'Hot GPUs',
  '总显存用量': 'Total memory usage',
  '总功耗': 'Total power',
  '当前功率': 'Current power',
  '{range} 耗电': '{range} energy',
  '估算电费': 'Estimated cost',
  '覆盖率': 'Coverage',
  'GPU 数量': 'GPU count',
  '平均利用率': 'Average utilization',
  'GPU 详细状态': 'GPU Details',
  '卡片化查看多设备 GPU 运行状态': 'Card view for multi-device GPU runtime',
  '暂无 GPU 上报': 'No GPU reports yet',
  '离线': 'Offline',
  'GPU 利用率': 'GPU utilization',
  '最近 1 小时': 'Last 1 hour',
  '显存': 'Memory',
  '总量 {value}': 'Total {value}',
  '温度': 'Temperature',
  '功耗': 'Power',
  '上限 {value}': 'Limit {value}',
  '利用率分布': 'Utilization Distribution',
  '当前快照': 'Current snapshot',
  '峰值 {value}': 'Peak {value}',
  '空闲 GPU': 'Idle GPUs',
  '活跃 GPU': 'Active GPUs',
  'PCIe 降级': 'PCIe degraded',
  '限速 GPU': 'Throttled GPUs',
  '功率与热状态': 'Power & Thermal State',
  '能源诊断': 'Energy Diagnostics',
  '当前范围未发现异常': 'No issues in current range',
  '当前范围未发现高温、限速或空转高耗': 'No thermal, throttle, or high-idle-power issues in current range',
  'GPU 能耗排行': 'GPU Energy Ranking',
  '只读': 'Read-only',
  '耗电': 'Energy',
  '电费': 'Cost',
  '空转': 'Idle',
  '空转高耗': 'High idle power',
  '暂无能耗数据': 'No energy data',
  '利用率 {value}': 'Utilization {value}',
  '{label} 历史趋势图': '{label} history trend chart',
  '过热': 'Overheated',
  '偏高': 'Elevated',
  '正常': 'Normal',
  '刚刚': 'Just now',
  '{value}s 前': '{value}s ago',
  '{value}m 前': '{value}m ago',
  '{value}h 前': '{value}h ago',
  '采集异常': 'Collection error',
  '高温': 'Hot',
  '关注': 'Watch',
  '显存空闲': 'Memory free',
  '显存保留': 'Memory reserved',
  '显存利用': 'Memory utilization',
  '温度上限': 'Temperature limit',
  '显存温度': 'Memory temperature',
  '功耗上限': 'Power limit',
  '风扇': 'Fan',
  '图形时钟': 'Graphics clock',
  '显存时钟': 'Memory clock',
  'SM 时钟': 'SM clock',
  '视频时钟': 'Video clock',
  'PCIe 当前': 'PCIe current',
  'PCIe 最大': 'PCIe max',
  '显示': 'Display',
  '驱动模型': 'Driver model',
  '新设备密钥': 'New device secret',
  '已创建设备 {name}': 'Created device {name}',
  '设备已启用': 'Device enabled',
  '设备已禁用': 'Device disabled',
  '已轮换密钥': 'Secret rotated',
  '设备密钥已轮换': 'Device secret rotated',
  '设备已删除': 'Device deleted',
  '设备名称已更新为 {name}': 'Device name updated to {name}',
  '注册设备': 'Register Device',
  '设备别名': 'Device alias',
  '创建中': 'Creating',
  '创建': 'Create',
  '设备列表': 'Device List',
  '设备名称': 'Device name',
  '保存': 'Save',
  '修改设备名称': 'Rename device',
  '改名': 'Rename',
  '禁用设备': 'Disable device',
  '启用设备': 'Enable device',
  '禁用': 'Disable',
  '启用': 'Enable',
  '轮换密钥': 'Rotate secret',
  '轮换': 'Rotate',
  '删除设备': 'Delete device',
  '删除': 'Delete',
  '暂无设备': 'No devices',
  '目标设备': 'Target device',
  '处理中': 'Processing',
  '允许 {name} 使用现有密钥继续上报 GPU 指标。': 'Allow {name} to continue reporting GPU metrics with the existing secret.',
  '确认启用': 'Confirm enable',
  '禁用后 {name} 的上报请求会被服务端拒绝，客户端本机配置不会被修改。': 'After disabling, reports from {name} will be rejected. Local client configuration is not changed.',
  '确认禁用': 'Confirm disable',
  '旧密钥会立即失效，需要在 {name} 所在机器手动更新新密钥后才能继续上报。': 'The old secret expires immediately. Update the new secret on {name} before it can report again.',
  '确认轮换': 'Confirm rotate',
  '删除后 {name} 将从设备列表和最新 GPU 快照中移除，原 Agent 密钥会失效。': 'After deletion, {name} is removed from the device list and latest GPU snapshots. The old Agent secret expires.',
  '确认删除': 'Confirm delete',
  '复制密钥': 'Copy secret',
  '已复制': 'Copied',
  '复制': 'Copy',
  'GPU 进程': 'GPU Processes',
  '暂无 GPU 进程快照': 'No GPU process snapshots',
  '24 小时统计': '24-hour Stats',
  '统计范围': 'Stats range',
  '筛选': 'Filter',
  '排序': 'Sort',
  '全部': 'All',
  '高负载': 'High load',
  '高空闲': 'High idle',
  '按平均利用率': 'By average utilization',
  '按峰值利用率': 'By peak utilization',
  '按空闲率': 'By idle rate',
  '按峰值显存': 'By peak memory',
  '按峰值温度': 'By peak temperature',
  '按峰值功耗': 'By peak power',
  '按样本数': 'By samples',
  '峰值利用率': 'Peak utilization',
  '空闲': 'Idle',
  '显存 均/峰': 'Memory avg/peak',
  '无匹配统计': 'No matching stats',
  '{range} 统计': '{range} Stats',
  '过去 {range} 曲线': 'Past {range} charts',
  '曲线加载失败': 'Chart loading failed',
  '暂无曲线数据': 'No chart data',
  '服务状态': 'Service Status',
  '等待服务端配置': 'Waiting for server configuration',
  '需要重启': 'Restart required',
  '当前协议': 'Current protocol',
  '证书已配置': 'Certificate configured',
  '未启用证书': 'Certificate not enabled',
  '证书到期': 'Certificate expiry',
  '未配置': 'Not configured',
  'HTTPS 下次启动生效': 'HTTPS takes effect on next start',
  'HTTPS 已启用': 'HTTPS enabled',
  'HTTP 模式': 'HTTP mode',
  '磁盘预留': 'Disk reserve',
  '空闲 {value}': '{value} free',
  '访问与安全': 'Access & Security',
  '凭据、端口、语言和 HTTPS 证书': 'Credentials, port, language, and HTTPS certificate',
  '重新打开端口、密码、语言和证书配置流程': 'Reopen the port, password, language, and certificate setup flow',
  '打开引导': 'Open wizard',
  '旧版 Agent 兼容': 'Legacy Agent Compatibility',
  '旧版 Agent 兼容说明': 'Legacy Agent compatibility details',
  '旧版兼容已开启': 'Legacy compatibility enabled',
  '旧版兼容已关闭': 'Legacy compatibility disabled',
  '旧版 Agent 兼容已开启': 'Legacy Agent compatibility enabled',
  '旧版 Agent 兼容已关闭': 'Legacy Agent compatibility disabled',
  '控制是否接受 0.1.9 前的 HMAC 签名': 'Controls whether pre-0.1.9 HMAC signatures are accepted',
  '仅建议在迁移旧 Agent 时临时开启。': 'Only enable this briefly while migrating old Agents.',
  '默认关闭，要求 Agent 使用绑定 device_id 的新签名。': 'Off by default; Agents must use new device_id-bound signatures.',
  '开启后，服务端会临时接受已登记且版本低于 0.1.9 的 Agent 旧 HMAC 签名；关闭后只接受绑定 device_id 的新签名。建议只在升级旧 Agent 的过渡期短时间开启。': 'When enabled, the server temporarily accepts legacy HMAC signatures from registered Agents older than 0.1.9. When disabled, only new device_id-bound signatures are accepted. Enable it only briefly while upgrading old Agents.',
  'Agent 自动更新': 'Agent Auto Update',
  'Agent 更新策略': 'Agent Update Policy',
  'Agent 更新策略说明': 'Agent update policy details',
  'Agent 拉取签名更新并替换自身': 'Agents pull signed updates and replace themselves',
  '客户端拉取签名 manifest 后自更新': 'Clients self-update after pulling signed manifests',
  'Agent 自更新已开启': 'Agent self-update enabled',
  'Agent 自更新已关闭': 'Agent self-update disabled',
  '更新范围': 'Update scope',
  '先更新 1 台，成功后继续': 'Update 1 Agent first, then continue after success',
  '先更新 {count} 台，成功后继续': 'Update {count} Agents first, then continue after success',
  '所有 Agent 按检查周期拉取': 'All Agents pull on their check interval',
  '高级设置': 'Advanced settings',
  '指定目标版本': 'Specific target version',
  '目标版本': 'Target version',
  '更新模式': 'Update mode',
  '仅通知': 'Notify only',
  '补丁版本': 'Patch versions',
  '小版本': 'Minor versions',
  '检查间隔秒': 'Check interval seconds',
  '并发上限': 'Max parallel',
  'Ed25519 公钥': 'Ed25519 public key',
  '先更新一批，成功后继续': 'Update one batch first, then continue after success',
  '全部 Agent 自行拉取': 'All Agents pull updates themselves',
  '留空表示最新补丁': 'Leave blank for the latest patch',
  'base64 编码公钥': 'base64-encoded public key',
  '保存策略': 'Save policy',
  'Agent 自动更新已保存': 'Agent auto update saved',
  'Agent 自动更新已关闭': 'Agent auto update disabled',
  'Agent 更新策略已保存': 'Agent update policy saved',
  'Agent 更新策略已关闭': 'Agent update policy disabled',
  '需要先配置签名更新源：请在高级设置填写 Manifest URL 和 Ed25519 公钥，或由部署环境预置默认更新源。': 'Configure a signed update source first: fill in Manifest URL and Ed25519 public key in Advanced settings, or preconfigure a default update source in the deployment environment.',
  '启用后，Agent 会定期用 HMAC 拉取更新策略，自行下载签名 manifest、校验 Ed25519 签名和 artifact sha256，再只替换自己的二进制。服务端不会下发 shell 命令。': 'When enabled, Agents periodically fetch update policies with HMAC, download signed manifests themselves, verify the Ed25519 signature and artifact sha256, then replace only their own binary. The server never sends shell commands.',
  '维护与发布': 'Maintenance & Release',
  '数据库、在线更新和版本信息': 'Database, online update, and release information',
  '能耗展示': 'Energy Display',
  '电费估算和热诊断阈值': 'Cost estimates and thermal diagnostic thresholds',
  '电价 / kWh': 'Price / kWh',
  '货币': 'Currency',
  '高温阈值 °C': 'Hot threshold °C',
  '空转利用率 %': 'Idle utilization %',
  '空转功率 W': 'Idle power W',
  '保存展示参数': 'Save display settings',
  '能耗展示参数已保存': 'Energy display settings saved',
  '电价不能为负数': 'Energy price cannot be negative',
  '高温阈值需在 1-120°C': 'Hot threshold must be 1-120°C',
  '空转利用率需在 0-100%': 'Idle utilization must be 0-100%',
  '空转功率需在 0-2000W': 'Idle power must be 0-2000W',
  '重启服务': 'Restart service',
  '访客功能': 'Guest access',
  '访客访问': 'Guest access',
  '访客总览': 'Guest overview',
  '访客视图': 'Guest view',
  '访客记录': 'Guest records',
  '允许访客访问': 'Allow guest access',
  '登录页显示访客入口，仅开放脱敏总览': 'The login page shows a guest entry and only exposes a sanitized overview.',
  '关闭后访客入口和访客总览不可访问': 'Guest entry and guest overview are unavailable when disabled.',
  '已开启': 'Enabled',
  '已关闭': 'Disabled',
  '访客功能已开启': 'Guest access enabled',
  '访客功能已关闭': 'Guest access disabled',
  '记录最近 100 次访客总览访问': 'Shows the latest 100 guest overview visits.',
  '暂无访客记录': 'No guest records yet',
  '关闭': 'Close',
  '确认重启服务端？': 'Restart the server?',
  '服务端会立即调度重启，页面将全屏等待服务恢复，恢复后自动刷新并提示重启成功。': 'The server will schedule a restart immediately. The page will wait full-screen for recovery, refresh automatically, and show a restart success notice.',
  '确认重启': 'Confirm restart',
  '服务端正在重启，恢复后页面会自动刷新。': 'Server is restarting. The page will refresh automatically after recovery.',
  '服务已重启': 'Service restarted',
  '服务端已重启并刷新页面。': 'The server has restarted and the page has been refreshed.',
  '已发送重启请求': 'Restart request sent',
  '服务端正在停止当前进程': 'Server is stopping the current process',
  '正在重启服务端': 'Restarting server',
  '页面正在等待服务恢复': 'The page is waiting for the service to recover',
  '更新已构建完成，服务端正在自动重启': 'Update built. Server is restarting automatically',
  '更新已构建完成，服务端正在自动重启。恢复后页面会自动刷新。': 'Update built. Server is restarting automatically. The page will refresh automatically after recovery.',
  '更新已构建完成，服务端正在自动重启，预计 {date} 前后恢复。恢复后页面会自动刷新。': 'Update built. Server is restarting automatically, expected to recover around {date}. The page will refresh automatically after recovery.',
  '，预计 {date} 前后恢复': ', expected to recover around {date}',
  '更新已拉取并构建完成，正在等待服务端重启': 'Update pulled and built. Waiting for server restart.',
  '恢复后页面会自动刷新。': 'The page will refresh automatically after recovery.',
  '版本已更新': 'Version updated',
  '自动更新已完成': 'Automatic update completed',
  '服务端已自动完成更新并重启。': 'The server completed the update and restarted automatically.',
  '更新时间': 'Updated at',
  '更新内容': 'Update notes',
  '无更新说明': 'No update notes',
  '服务端已自动重启并刷新页面。': 'The server restarted automatically and the page has been refreshed.',
  'HTTPS 证书已启用': 'HTTPS certificate enabled',
  'HTTPS 证书已保存，服务端已自动重启并刷新页面。': 'HTTPS certificate saved. The server restarted automatically and the page has been refreshed.',
  '知道了': 'OK',
  '当前已经是最新版本': 'Already up to date',
  '在线更新': 'Online Update',
  '未绑定上游': 'No upstream',
  '检查 Git 上游版本': 'Check Git upstream version',
  '当前提交': 'Current commit',
  '远端提交': 'Remote commit',
  '运行版本': 'Running version',
  '仓库版本': 'Repository version',
  '运行提交': 'Running commit',
  '落后': 'Behind',
  '超前': 'Ahead',
  '远端': 'Remote',
  '检查时间': 'Checked at',
  '检查中': 'Checking',
  '重启中': 'Restarting',
  '检查更新': 'Check update',
  '更新': 'Update',
  '更新中': 'Updating',
  '确认更新服务端？': 'Confirm server update?',
  '工作区不干净，是否强制更新？': 'Working tree is dirty. Force update?',
  '服务端会检查依赖、构建远端提交、执行 fast-forward 拉取，并在成功后自动重启。重启期间页面会显示进度并等待服务恢复。': 'The server will check dependencies, build the remote commit, fast-forward pull, and restart automatically after success. Progress is shown while the page waits for recovery.',
  '服务端会先用 git stash push -u 保存当前工作区改动，再检查依赖、构建远端提交、执行 fast-forward 拉取并自动重启。': 'The server will first save current worktree changes with git stash push -u, then check dependencies, build the remote commit, fast-forward pull, and restart automatically.',
  '确认更新': 'Confirm update',
  '暂存并更新': 'Stash and update',
  '拉取并重启': 'Pull and restart',
  '重建并重启': 'Rebuild and restart',
  '更新代理': 'Update proxy',
  '保存代理': 'Save proxy',
  '更新代理已保存': 'Update proxy saved',
  '更新代理已清空': 'Update proxy cleared',
  '自动更新已开启': 'Automatic update enabled',
  '自动更新已关闭': 'Automatic update disabled',
  '每 30 分钟检查一次，有更新时自动拉取、构建并重启': 'Checks every 30 minutes and automatically pulls, builds, and restarts when an update is available',
  '每 1 小时检查一次，有更新时在设置入口提示': 'Checks every hour and flags Settings when an update is available',
  '已发送更新请求': 'Update request sent',
  '依赖预检、构建远端提交并执行 fast-forward 拉取': 'Checking dependencies, building the remote commit, and fast-forward pulling',
  '更新已应用，准备自动重启': 'Update applied. Preparing automatic restart.',
  '服务端正在自动重启': 'Server is restarting automatically',
  '服务端正在自动重启，恢复后页面会自动刷新。': 'Server is restarting automatically. The page will refresh automatically after recovery.',
  '等待服务端恢复，恢复后自动刷新': 'Waiting for the server to recover. The page will refresh automatically.',
  '正在读取 Git 状态': 'Reading Git status',
  '失败': 'Failed',
  '未知': 'Unknown',
  '尚未读取更新状态': 'Update status has not been read',
  '不可用': 'Unavailable',
  '服务端未运行在 Git 工作区': 'Server is not running in a Git working tree',
  '需确认': 'Confirmation needed',
  '已阻止': 'Blocked',
  '服务端工作区存在未提交改动；自动拉取已阻止，手动更新可先暂存改动后继续': 'Server working tree has uncommitted changes. Automatic pull is blocked; manual update can stash the changes first and continue.',
  '来源异常': 'Source issue',
  '自动更新来源校验未通过': 'Automatic update source validation failed',
  '未绑定': 'Unbound',
  '当前分支没有 Git upstream': 'Current branch has no Git upstream',
  '分叉': 'Diverged',
  '本地和上游存在分叉，不能自动 fast-forward': 'Local and upstream have diverged. Fast-forward is unavailable.',
  '本地超前': 'Local ahead',
  '本地提交超前上游，面板不会执行拉取': 'Local commits are ahead of upstream. The dashboard will not pull.',
  '有新版本': 'Update available',
  '需重建': 'Rebuild needed',
  '运行中的服务端二进制与当前仓库不一致，可重建并自动重启': 'The running server binary does not match the current repository checkout. It can be rebuilt and restarted automatically.',
  '{count} 个提交可拉取、构建并自动重启': '{count} commits can be pulled, built, and restarted automatically',
  '最新': 'Latest',
  '已经是最新版本': 'Already up to date',
  '加载中': 'Loading',
  '版本与变更': 'Version & Changes',
  'GPUFleet 发布信息': 'GPUFleet release information',
  '作者': 'Author',
  '版本': 'Version',
  '提交': 'Commit',
  '构建时间': 'Build time',
  '仓库地址': 'Repository',
  '最近变更': 'Latest changes',
  '更多更新记录': 'More changelog',
  '正在读取版本信息': 'Reading release information',
  '打开 GitHub': 'Open GitHub',
  '新增': 'Added',
  '变更': 'Changed',
  '安全': 'Security',
  '修复': 'Fixed',
  '密码更改': 'Password Change',
  '仅使用密码作为 Web 凭据': 'Password is the only web credential',
  '当前密码': 'Current password',
  '密码已更新': 'Password updated',
  '更新密码': 'Update password',
  '端口配置': 'Port Configuration',
  '当前监听端口': 'Current listening port',
  '端口已保存，重启后生效': 'Port saved. Restart to apply.',
  '端口已保存': 'Port saved',
  '保存端口': 'Save port',
  '语言设置': 'Language Settings',
  '控制首次配置、面板和后续设置页语言': 'Controls first-time setup, dashboard, and settings language',
  '语言已保存': 'Language saved',
  '保存语言': 'Save language',
  '证书已保存，重启后启用 HTTPS': 'Certificate saved. Restart to enable HTTPS.',
  '证书已保存，服务端正在自动重启。恢复后页面会自动刷新。': 'Certificate saved. The server is restarting automatically. The page will refresh after recovery.',
  '证书已保存': 'Certificate saved',
  '到期 {date}': 'Expires {date}',
  '证书文件': 'Certificate file',
  '选择文件': 'Choose file',
  '未选择文件': 'No file selected',
  '上传中': 'Uploading',
  '上传证书': 'Upload certificate',
  '数据库下载': 'Database Download',
  '数据库大小': 'Database size',
  '数据库大小 {size} · 已存储 {days} 天 · {free} 空闲': 'Database size {size} · stored {days} days · {free} free',
  '下载数据库': 'Download database',
  '下载诊断包': 'Download diagnostics',
  '预留空间 MiB': 'Reserved space MiB',
  '保存预留': 'Save reserve',
  '磁盘预留至少 64 MiB': 'Disk reserve must be at least 64 MiB',
  '磁盘预留已保存': 'Disk reserve saved',
  '查看 Git 原始错误': 'View raw Git error',
  'Git 原始错误': 'Raw Git error',
  '用于诊断服务器网络、代理或 Git 上游问题。': 'Use this to diagnose server network, proxy, or Git upstream issues.',
  '在线更新失败，请查看详情并检查服务器网络、Git 上游或更新代理配置。': 'Online update failed. View details and check the server network, Git upstream, or update proxy settings.',
  '检查 Git 上游失败': 'Git upstream check failed',
  '请求过于频繁，请等待 {duration} 后再试': 'Too many requests. Retry after {duration}.',
  '检查失败': 'Check failed',
  '利用率': 'Utilization',
  '平均': 'Average',
  '峰值': 'Peak',
  '正在拉取并重启': 'Pulling and restarting',
  '请确认当前更新代理可由服务端访问。': 'Confirm the current update proxy is reachable from the server.',
  '请在设置页配置服务端可访问的更新代理，或检查服务器直连 GitHub 的网络。': 'Configure an update proxy reachable from the server, or check direct server connectivity to GitHub.',
  '当前服务端未包含更新代理接口，请先完成服务端更新并重启': 'The current server does not include the update proxy endpoint. Finish the server update and restart first.',
  '在线更新失败：GitHub TLS 连接被中断。{hint}': 'Online update failed: the GitHub TLS connection was interrupted. {hint}',
  '在线更新失败：服务器连接 GitHub 超时或被拒绝。{hint}': 'Online update failed: the server connection to GitHub timed out or was refused. {hint}',
  '在线更新失败：服务器无法解析 GitHub 域名。请检查 DNS、网络或更新代理。': 'Online update failed: the server cannot resolve the GitHub domain. Check DNS, network connectivity, or the update proxy.',
  '在线更新失败：远端仓库认证失败。请检查仓库地址、访问权限或凭据配置。': 'Online update failed: remote repository authentication failed. Check the repository URL, access permissions, or credentials.',
  'language endpoint not found; rebuild and restart the server binary': 'Language endpoint not found; rebuild and restart the server binary.'
};

const duplicateEnglish = new Set<string>();
const zhByEn = Object.entries(en).reduce<Record<string, string>>((out, [zh, translated]) => {
  if (duplicateEnglish.has(translated)) {
    return out;
  }
  if (Object.prototype.hasOwnProperty.call(out, translated)) {
    delete out[translated];
    duplicateEnglish.add(translated);
  } else {
    out[translated] = zh;
  }
  return out;
}, {});

const enPatterns: Array<[RegExp, string]> = [
  [/^(\d+) 台设备，(\d+) 块 GPU，按最新上报状态汇总。$/, '$1 devices, $2 GPUs, summarized from the latest reports.'],
  [/^(\d+) 块 GPU$/, '$1 GPUs'],
  [/^(\d+) 个样本$/, '$1 samples'],
  [/^(\d+) 项$/, '$1 items'],
  [/^先更新 (\d+) 台，成功后继续$/, 'Update $1 Agents first, then continue after success'],
  [/^服务端时间 (.+)$/, 'Server time $1'],
  [/^总量 (.+)$/, 'Total $1'],
  [/^上限 (.+)$/, 'Limit $1'],
  [/^峰值 (.+)$/, 'Peak $1'],
  [/^数据库大小 (.+) · 已存储 (.+) 天 · (.+) 空闲$/, 'Database size $1 · stored $2 days · $3 free'],
  [/^数据库大小 (.+)$/, 'Database size $1'],
  [/^空闲 (.+)$/, '$1 free'],
  [/^当前证书到期：(.+)$/, 'Current certificate expires: $1'],
  [/^到期 (.+)$/, 'Expires $1'],
  [/^已创建设备 (.+)$/, 'Created device $1'],
  [/^设备名称已更新为 (.+)$/, 'Device name updated to $1'],
  [/^允许 (.+) 使用现有密钥继续上报 GPU 指标。$/, 'Allow $1 to continue reporting GPU metrics with the existing secret.'],
  [/^禁用后 (.+) 的上报请求会被服务端拒绝，客户端本机配置不会被修改。$/, 'After disabling, reports from $1 will be rejected. Local client configuration is not changed.'],
  [/^旧密钥会立即失效，需要在 (.+) 所在机器手动更新新密钥后才能继续上报。$/, 'The old secret expires immediately. Update the new secret on $1 before it can report again.'],
  [/^删除后 (.+) 将从设备列表和最新 GPU 快照中移除，原 Agent 密钥会失效。$/, 'After deletion, $1 is removed from the device list and latest GPU snapshots. The old Agent secret expires.'],
  [/^(\d+) 个提交可拉取、构建并自动重启$/, '$1 commits can be pulled, built, and restarted automatically'],
  [/^更新已构建完成，服务端正在自动重启，预计 (.+) 前后恢复$/, 'Update built. Server is restarting automatically, expected to recover around $1'],
  [/^更新已构建完成，服务端正在自动重启，预计 (.+) 前后恢复。恢复后页面会自动刷新。$/, 'Update built. Server is restarting automatically, expected to recover around $1. The page will refresh automatically after recovery.'],
  [/^(\d+) 天$/, '$1 days'],
  [/^(\d+) 天 (\d+) 小时$/, '$1 days $2 hours'],
  [/^(\d+) 小时$/, '$1 hours'],
  [/^(.+) 历史趋势图$/, '$1 history trend chart']
];

const textSources = new WeakMap<Text, string>();
const attrSources = new WeakMap<Element, Record<string, string>>();
let observer: MutationObserver | undefined;

export function makeTranslator(language: AppLanguage): Translate {
  return (key, values) => {
    const template = language === 'en-US' ? en[key] ?? key : key;
    if (!values) return template;
    return Object.entries(values).reduce(
      (text, [name, value]) => text.split(`{${name}}`).join(String(value)),
      template
    );
  };
}

export function translateText(value: string, language: AppLanguage): string {
  const trimmed = value.trim();
  if (!trimmed) return value;
  if (language === 'zh-CN') {
    const source = zhByEn[trimmed];
    return source ? value.replace(trimmed, source) : value;
  }
  const exact = en[trimmed];
  if (exact) return value.replace(trimmed, exact);
  for (const [pattern, replacement] of enPatterns) {
    if (pattern.test(trimmed)) return value.replace(trimmed, trimmed.replace(pattern, replacement));
  }
  return value;
}

export function installDOMI18n(getLanguage: () => AppLanguage) {
  const apply = () => translateRoot(document.body, getLanguage());
  observer?.disconnect();
  observer = new MutationObserver(() => apply());
  apply();
  observer.observe(document.body, {
    childList: true,
    subtree: true,
    characterData: true,
    attributes: true,
    attributeFilter: ['title', 'placeholder', 'aria-label']
  });
  return () => observer?.disconnect();
}

export function translateRoot(root: ParentNode, language: AppLanguage) {
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
  const textNodes: Text[] = [];
  while (walker.nextNode()) textNodes.push(walker.currentNode as Text);
  for (const node of textNodes) {
    if (!node.parentElement || node.parentElement.closest('script,style,code')) continue;
    const stored = textSources.get(node);
    const current = node.nodeValue ?? '';
    let source = stored ?? current;
    if (stored) {
      const translatedStored = translateText(stored, language);
      if (current !== stored && current !== translatedStored) source = current;
    }
    if (!textSources.has(node) || source !== stored) textSources.set(node, source);
    const next = translateText(source, language);
    if (node.nodeValue !== next) node.nodeValue = next;
  }
  const elements = root instanceof Element ? [root, ...Array.from(root.querySelectorAll('*'))] : Array.from(root.querySelectorAll('*'));
  for (const element of elements) {
    if (element.closest('script,style,code')) continue;
    const stored = attrSources.get(element) ?? {};
    let changed = false;
    for (const attr of ['title', 'placeholder', 'aria-label']) {
      const current = element.getAttribute(attr);
      if (!current) continue;
      let source = stored[attr] ?? current;
      if (stored[attr]) {
        const translatedStored = translateText(stored[attr], language);
        if (current !== stored[attr] && current !== translatedStored) source = current;
      }
      stored[attr] = source;
      const next = translateText(source, language);
      if (current !== next) element.setAttribute(attr, next);
      changed = true;
    }
    if (changed) attrSources.set(element, stored);
  }
}

export type I18nContextValue = {
  language: AppLanguage;
  setLanguage: (language: AppLanguage) => void;
  t: Translate;
};

export const I18nContext = createContext<I18nContextValue>({
  language: 'zh-CN',
  setLanguage: () => undefined,
  t: makeTranslator('zh-CN')
});

export function useI18n() {
  return useContext(I18nContext);
}
