package version

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	Version   = "0.1.7"
	Commit    = "dev"
	BuildTime = ""
)

const (
	Product    = "GPUFleet"
	Author     = "stlin256"
	Repository = "https://github.com/stlin256/GPU-Fleet"
)

type ReleaseInfo struct {
	Product    string           `json:"product"`
	Version    string           `json:"version"`
	Commit     string           `json:"commit"`
	BuildTime  string           `json:"build_time,omitempty"`
	Author     string           `json:"author"`
	Repository string           `json:"repository"`
	Changelog  []ChangelogEntry `json:"changelog"`
}

type ChangelogEntry struct {
	Version    string   `json:"version"`
	Date       string   `json:"date"`
	Title      string   `json:"title"`
	TitleEN    string   `json:"title_en,omitempty"`
	Added      []string `json:"added,omitempty"`
	AddedEN    []string `json:"added_en,omitempty"`
	Changed    []string `json:"changed,omitempty"`
	ChangedEN  []string `json:"changed_en,omitempty"`
	Security   []string `json:"security,omitempty"`
	SecurityEN []string `json:"security_en,omitempty"`
	Fixed      []string `json:"fixed,omitempty"`
	FixedEN    []string `json:"fixed_en,omitempty"`
}

func Current() ReleaseInfo {
	return ReleaseInfo{
		Product:    Product,
		Version:    Version,
		Commit:     Commit,
		BuildTime:  BuildTime,
		Author:     Author,
		Repository: Repository,
		Changelog:  Changelog(),
	}
}

func CurrentFromChangelog(path string) ReleaseInfo {
	info := Current()
	if entries, err := ChangelogFromFile(path); err == nil && len(entries) > 0 {
		info.Changelog = entries
	}
	return info
}

func String() string {
	if Commit == "" || Commit == "dev" {
		return fmt.Sprintf("%s %s", Product, Version)
	}
	return fmt.Sprintf("%s %s (%s)", Product, Version, Commit)
}

func ChangelogFromFile(path string) ([]ChangelogEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	entries := parseChangelogMarkdown(string(raw))
	if len(entries) == 0 {
		return nil, fmt.Errorf("no changelog entries found in %s", path)
	}
	return entries, nil
}

var changelogVersionHeading = regexp.MustCompile(`^## \[([0-9]+\.[0-9]+\.[0-9]+)\] - ([0-9]{4}-[0-9]{2}-[0-9]{2})$`)

func parseChangelogMarkdown(raw string) []ChangelogEntry {
	var entries []ChangelogEntry
	var current *ChangelogEntry
	section := ""
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if match := changelogVersionHeading.FindStringSubmatch(line); match != nil {
			if current != nil {
				entries = append(entries, *current)
			}
			current = &ChangelogEntry{Version: match[1], Date: match[2]}
			section = ""
			continue
		}
		if current == nil {
			continue
		}
		if strings.HasPrefix(line, "### ") {
			section = normalizeChangelogSection(strings.TrimSpace(strings.TrimPrefix(line, "### ")))
			continue
		}
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		lang, text := parseLocalizedBullet(strings.TrimSpace(strings.TrimPrefix(line, "- ")))
		appendChangelogText(current, section, lang, text)
	}
	if current != nil {
		entries = append(entries, *current)
	}
	return entries
}

func normalizeChangelogSection(section string) string {
	lower := strings.ToLower(section)
	switch {
	case strings.Contains(lower, "title"), strings.Contains(section, "标题"):
		return "title"
	case strings.Contains(lower, "added"), strings.Contains(section, "新增"):
		return "added"
	case strings.Contains(lower, "changed"), strings.Contains(section, "变更"):
		return "changed"
	case strings.Contains(lower, "security"), strings.Contains(section, "安全"):
		return "security"
	case strings.Contains(lower, "fixed"), strings.Contains(section, "修复"):
		return "fixed"
	default:
		return ""
	}
}

func parseLocalizedBullet(raw string) (string, string) {
	for _, prefix := range []string{"zh-CN:", "zh:", "中文:"} {
		if strings.HasPrefix(raw, prefix) {
			return "zh-CN", strings.TrimSpace(strings.TrimPrefix(raw, prefix))
		}
	}
	for _, prefix := range []string{"en-US:", "en:", "English:"} {
		if strings.HasPrefix(raw, prefix) {
			return "en-US", strings.TrimSpace(strings.TrimPrefix(raw, prefix))
		}
	}
	if containsCJK(raw) {
		return "zh-CN", raw
	}
	return "en-US", raw
}

func appendChangelogText(entry *ChangelogEntry, section, lang, text string) {
	if text == "" {
		return
	}
	if section == "title" {
		if lang == "en-US" {
			entry.TitleEN = text
		} else {
			entry.Title = text
		}
		return
	}
	switch section {
	case "added":
		if lang == "en-US" {
			entry.AddedEN = append(entry.AddedEN, text)
		} else {
			entry.Added = append(entry.Added, text)
		}
	case "changed":
		if lang == "en-US" {
			entry.ChangedEN = append(entry.ChangedEN, text)
		} else {
			entry.Changed = append(entry.Changed, text)
		}
	case "security":
		if lang == "en-US" {
			entry.SecurityEN = append(entry.SecurityEN, text)
		} else {
			entry.Security = append(entry.Security, text)
		}
	case "fixed":
		if lang == "en-US" {
			entry.FixedEN = append(entry.FixedEN, text)
		} else {
			entry.Fixed = append(entry.Fixed, text)
		}
	}
}

func containsCJK(value string) bool {
	for _, r := range value {
		if (r >= '\u4e00' && r <= '\u9fff') || (r >= '\u3400' && r <= '\u4dbf') {
			return true
		}
	}
	return false
}

func Changelog() []ChangelogEntry {
	entries := []ChangelogEntry{
		{
			Version: "0.1.7",
			Date:    "2026-06-08",
			Title:   "安装更新、GPU 监控、设置与存储优化",
			TitleEN: "Installation, update, GPU monitoring, settings, and storage improvements",
			Changed: []string{
				"Linux 自动更新重启脚本改为先将新二进制原子替换到当前路径，再等待旧进程退出，避免 systemd 在替换前抢先拉起旧二进制。",
				"重启脚本会检测当前二进制路径是否已被其他进程启动，避免 systemd 场景下重复拉起两个服务端进程。",
				"GPU 详情和总览卡片布局进一步压缩，长型号、趋势标题、功耗/显存说明和 Compute 信息不再挤压卡片内容。",
				"趋势卡片主数值优先保持单行显示并占满整行，避免功耗、显存等指标在仍有空间时被拆成多行或显示省略号。",
				"GPU 进程和 24 小时统计列表元信息优先显示设备名称，减少直接展示设备 ID。",
				"总览和 GPU 监控页的汇总指标卡新增右侧迷你曲线，用于展示各 GPU 当前利用率、显存和功耗分布。",
				"在线更新进度改为背景模糊加前景进度面板展示，并加入百分比、进度条和阶段动画以提升更新体验。",
				"24 小时统计列表支持点击 GPU 展开过去 24H 的利用率、显存、温度和功耗曲线，GPU 监控页统计面板宽度与详情卡片主列对齐。",
				"版本 API 和设置页 Changelog 改为优先读取仓库中的 CHANGELOG.md，并规范化中英文条目格式。",
				"新增 Linux 服务端一键安装脚本，支持 clone 后构建、写入 systemd 服务、开机自启动并使用当前仓库作为在线更新来源。",
				"新增双语安装指南，覆盖 Linux 服务端、旧部署升级、Linux/Windows/WSL2 设备端 Agent、服务命令、卸载和连通性检查。",
				"优化移动端顶部 GPU 指标卡布局，带迷你曲线的总显存用量和总功耗卡片不再挤压文字，GPU 页顶部卡片在小屏保持更紧凑的双列展示。",
				"GPU 详细状态的参数网格在移动端保持两列紧凑展示，减少单张 GPU 卡片的纵向占用。",
				"在线更新卡片移除超前、运行提交和远端字段，更新按钮改为二次确认后执行，并新增可立即生效的磁盘预留空间设置。",
				"设置页数据库下载卡片改为显示实际已存储天数，并将 7 天外指标分段重压缩为单个高压缩率 gzip 成员以降低长期存储占用。",
			},
			ChangedEN: []string{
				"Linux update restart helpers now move the new binary into the active path before waiting for the old process to exit, preventing systemd from restarting the old binary first.",
				"The restart helper detects whether another process is already running the target binary path to avoid starting a duplicate server under systemd.",
				"GPU detail and overview card layouts are more compact so long model names, trend labels, power/memory captions, and Compute metadata no longer crowd the card contents.",
				"Trend card primary values now prefer single-line display and span the full row, avoiding wraps or ellipses while horizontal space remains available.",
				"GPU process and 24-hour stats metadata now prefer device names, reducing direct device ID display.",
				"Overview and GPU monitoring aggregate metric cards now include right-side sparklines for current per-GPU utilization, memory, and power distribution.",
				"Online update progress now uses a blurred backdrop with a foreground progress panel, percentage, progress bar, and staged animation for a clearer update experience.",
				"24-hour stats rows can now expand per GPU to show 24-hour utilization, memory, temperature, and power charts, with the GPU monitoring stats panel aligned to the detail-card column width.",
				"Version API and settings changelog now prefer reading CHANGELOG.md from the repository, with normalized bilingual entry formatting.",
				"Added a one-command Linux server installer that builds after clone, writes a systemd service, enables boot startup, and uses the current checkout as the online-update source.",
				"Added a bilingual installation guide covering Linux server setup, older deployment upgrades, Linux/Windows/WSL2 Agents, service commands, uninstall steps, and connectivity checks.",
				"Improved mobile GPU metric card layouts so memory and power sparklines no longer squeeze text, while GPU-page top cards stay in a more compact two-column layout on small screens.",
				"GPU detail cards now keep their parameter grid in a compact two-column layout on mobile, reducing vertical space per GPU card.",
				"The online update card now removes ahead, running commit, and remote fields, requires confirmation before updating, and adds an immediately applied disk reserve setting.",
				"The settings database download card now shows actual stored days, and metric segments older than 7 days are recompressed into single high-compression gzip members to reduce long-term storage use.",
			},
			Fixed: []string{
				"修复 0.1.5 到后续版本自动更新时，Git 仓库已更新但 systemd 仍可能继续运行旧服务端二进制的问题。",
				"修复 GPU 监控页离线 GPU 卡片没有遮罩的问题，离线遮罩现在会同时显示离线时长。",
				"修复离线 GPU 重新上线后，总览和 GPU 监控页小图表可能继续沿用空曲线缓存的问题，并统一 GPU 详情数值悬浮提示样式。",
				"修复 GPU 最新快照变化时小图表反复重建刷新的问题，并将当前快照补入曲线末端，让总览和 GPU 监控页的总功耗曲线与当前数值保持一致。",
				"调小总览和 GPU 监控页顶部迷你图表悬浮提示的数值字号，避免提示层遮挡时过于突兀。",
			},
			FixedEN: []string{
				"Fixed automatic updates from 0.1.5 and later where the Git checkout updated but systemd could continue running the old server binary.",
				"Fixed missing offline masks on GPU monitoring cards; the mask now also shows how long the GPU has been offline.",
				"Fixed overview and GPU monitoring sparklines potentially reusing empty series caches after an offline GPU comes back online, and aligned GPU detail value hover tooltips with the chart tooltip style.",
				"Fixed sparklines repeatedly rebuilding as GPU snapshots changed, and appends the current snapshot to chart tails so overview and GPU monitoring power trends match the current value.",
				"Reduced the value font size in top metric sparkline hover tooltips on Overview and GPU monitoring so the overlay feels less intrusive.",
			},
		},
		{
			Version: "0.1.6",
			Date:    "2026-06-05",
			Title:   "自动更新二进制一致性检测",
			TitleEN: "Automatic update binary consistency detection",
			Added: []string{
				"在线更新状态新增运行版本、运行提交、仓库版本和二进制过期状态，用于区分 Git 工作区提交和当前运行的服务端二进制。",
				"当 Git 仓库已经是最新但运行二进制仍是旧版本时，更新面板会显示“需重建”，并允许执行重建并重启。",
			},
			AddedEN: []string{
				"Online update status now reports running version, running commit, repository version, and binary-outdated state to distinguish the Git checkout from the running server binary.",
				"When the Git repository is already current but the running binary is stale, the update panel shows a rebuild-needed state and allows rebuild-and-restart.",
			},
			Changed: []string{
				"自动更新构建服务端时写入 version.Commit 和 version.BuildTime，后续版本检查可准确判断当前运行二进制来源。",
				"无远端新提交但二进制过期时，服务端会从当前仓库提交构建新的二进制并自动重启，不再误判为已经是最新版本。",
			},
			ChangedEN: []string{
				"Automatic server builds now stamp version.Commit and version.BuildTime so later checks can identify the running binary source accurately.",
				"When no remote commit is pending but the binary is stale, the server rebuilds from the current repository commit and restarts automatically instead of reporting already up to date.",
			},
			Fixed: []string{
				"修复仓库已拉到最新但 /opt/gpufleet/gpufleet-server 仍是旧二进制时，在线更新页面显示“最新”且无法触发重建的问题。",
			},
			FixedEN: []string{
				"Fixed the update panel showing latest and blocking rebuild when the repository was current but /opt/gpufleet/gpufleet-server was still an old binary.",
			},
		},
		{
			Version: "0.1.5",
			Date:    "2026-06-05",
			Title:   "在线更新体验、HTTPS 证书重启与双语变更记录",
			TitleEN: "Online update UX, HTTPS certificate restart, and bilingual changelog",
			Added: []string{
				"在线更新新增代理地址配置，Git fetch、worktree 构建和 Go build 会复用该代理环境。",
				"拉取并重启流程新增明确进度反馈，覆盖请求发送、依赖预检、构建、重启和恢复等待。",
				"上传 HTTPS 证书后会自动调度服务端重启，恢复后页面自动刷新并弹出提示。",
				"版本 API 和设置页 Changelog 新增中英双语字段，英文模式显示英文变更内容。",
			},
			AddedEN: []string{
				"Added an online update proxy setting reused by Git fetch, update worktree builds, and Go build.",
				"Added explicit progress feedback for pull-and-restart: request, dependency checks, build, restart, and recovery wait.",
				"HTTPS certificate upload now schedules an automatic server restart, refreshes the page after recovery, and shows a notice.",
				"Version API and the settings changelog now expose bilingual Chinese and English fields.",
			},
			Changed: []string{
				"设置页在线更新状态会缓存 1 小时，打开设置页时优先显示缓存结果，并在后台按小时刷新。",
				"点击检查更新会绕过缓存并立即重新检查 Git upstream 状态。",
				"首页顶部 KPI 在 overview 尚未加载完成时显示占位符，避免短暂显示 0/0 等错误数值。",
				"README 顶部 Logo 调整为小图标加项目名称的横幅形式，避免首屏被大图占满。",
				"刷新 imgs 目录中的部署截图素材。",
			},
			ChangedEN: []string{
				"Online update status is cached for one hour, shown immediately on settings open, and refreshed hourly in the background.",
				"The Check update button bypasses the cache and rechecks Git upstream immediately.",
				"Top overview KPI cards show placeholders until overview data is loaded, avoiding transient 0/0 values.",
				"README logo presentation was changed to a compact logo plus project-name banner.",
				"Refreshed deployment screenshots in the imgs directory.",
			},
			Fixed: []string{
				"修复英文模式下 GPU 卡片相对时间仍显示“前”的问题。",
				"修复 HTTPS 已启用时仍提示“下次启动生效”的状态文案。",
				"修复数据库大小为空时设置页只显示短横线的问题。",
				"修复 HTTPS 证书上传文件选择控件在英文模式下仍显示浏览器原生中文文案的问题。",
			},
			FixedEN: []string{
				"Fixed GPU card relative time still showing the Chinese suffix in English mode.",
				"Fixed HTTPS status copy still saying it would take effect on next start after HTTPS was already active.",
				"Fixed database size showing only a dash when no size had been loaded.",
				"Fixed HTTPS certificate file pickers showing browser-native Chinese copy in English mode.",
			},
		},
		{
			Version: "0.1.4",
			Date:    "2026-06-05",
			Title:   "图表悬浮时间与数据库大小",
			Added: []string{
				"设置页数据库下载卡片新增数据库大小显示，对应可下载数据库内容的实际文件大小。",
			},
			Changed: []string{
				"GPU 趋势图悬浮提示从采集点序号改为显示采样时间。",
				"内置 fallback 面板同步显示采样时间和数据库大小，保持缺少 web/dist 时的体验一致。",
			},
		},
		{
			Version: "0.1.3",
			Date:    "2026-06-05",
			Title:   "更新重启反馈与默认设备修复",
			Changed: []string{
				"服务端在线更新成功并自动重启后，Web 面板会等待服务恢复、自动刷新页面并展示版本更新弹窗。",
				"语言保存接口缺失时，前端会提示需要重建并重启服务端，避免只显示 not found。",
				"服务端启动不再默认创建 local-dev 引导设备；只有显式配置 bootstrap device id 和 secret 时才会创建初始设备。",
			},
			Fixed: []string{
				"修复删除 local-dev 后，服务端自动更新或重启又重新创建该设备的问题。",
			},
		},
		{
			Version: "0.1.2",
			Date:    "2026-06-05",
			Title:   "服务端国际化框架",
			Added: []string{
				"新增服务端语言配置，支持首次配置时选择简体中文或 English，并持久化到 metadata.json。",
				"新增设置页语言切换能力，语言偏好会同步到服务端并立即影响 Web 面板。",
				"新增可扩展前端 i18n 词表和动态文案翻译兜底，覆盖 React 面板和内置 fallback 面板的主要用户可见文案。",
				"新增英文 README 和 i18n 维护文档，API、前端、运维和当前实现文档补充语言配置说明。",
			},
			Changed: []string{
				"首次配置流程扩展为密码、端口、语言和可选 HTTPS 证书的统一配置流程。",
				"服务状态 API、overview API 和设置相关响应现在返回当前语言字段，便于多浏览器保持一致界面语言。",
			},
			Fixed: []string{
				"补齐设置页、更新页、设备管理、指标卡片和错误提示等界面的中英文文案维护入口，降低后续新增语言时遗漏文案的风险。",
			},
		},
		{
			Version: "0.1.1",
			Date:    "2026-06-05",
			Title:   "设备管理与移动端体验增强",
			Added: []string{
				"设备管理支持改名和删除，删除后会清理该设备的最新 GPU 与进程快照。",
				"总览新增总功耗指标，并以 GiB 展示全局总显存用量。",
				"移动端 GPU 趋势图在小屏继续保持 2x2 布局，并压缩图表尺寸以减少滚动。",
			},
			Changed: []string{
				"设备页中的禁用、启用、删除和密钥轮换统一使用应用内确认弹窗。",
				"导航顺序调整为总览、GPU、设备、设置，优先进入多卡监控视角。",
				"设置页按访问与安全、维护与发布重新分组，保留密码、端口、证书、数据库、在线更新、配置引导和版本信息。",
				"服务端在线更新从单纯 fast-forward 拉取升级为依赖预检、远端提交构建、fast-forward 拉取和 Windows/Linux 自动重启。",
			},
			Security: []string{
				"高风险设备操作需要二次确认，降低误禁用、误删除和误轮换密钥风险。",
				"在线更新拒绝前端传入命令、路径、分支和远端；缺少 git、go 或平台重启器依赖时会在拉取前阻止更新。",
			},
			Fixed: []string{
				"修复服务设置页视觉混排和旧式指标文案导致的信息不清晰问题。",
			},
		},
		{
			Version: "0.1.0",
			Date:    "2026-06-03",
			Title:   "MVP 预览版：安全的多设备 GPU 可观测面板",
			Added: []string{
				"支持 Windows 和 Linux NVIDIA GPU 设备的客户端-服务端架构。",
				"React 面板提供多设备 GPU 卡片、历史图表、深浅主题、移动端底部导航和 SVG 品牌 Logo。",
				"首次启动交互式配置访问密码、端口和可选 HTTPS 证书。",
				"登录后的设置页可查看版本号、构建信息和变更记录。",
				"设置页可检查服务端 Git upstream，并在工作区干净且可 fast-forward 时拉取更新。",
			},
			Changed: []string{
				"设置页聚合项目署名、数据库下载、证书状态和发布信息。",
				"缺少 web/dist 时，内置 fallback 面板仍保留主要展示、在线更新和运维入口。",
			},
			Security: []string{
				"Agent 上报使用 HMAC 签名并带 nonce 重放保护。",
				"Web 登录仅使用密码凭据，并记住当前浏览器设备 30 天。",
				"登录入口具备短窗口限流和递进锁定的防爆破保护。",
				"服务端保持只接收数据，不暴露客户端命令或配置下发接口。",
				"在线更新接口只执行固定 Git 参数，拒绝 dirty、无 upstream、本地超前或分叉工作区。",
			},
		},
	}
	return append([]ChangelogEntry(nil), entries...)
}
