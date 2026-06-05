package version

import "fmt"

var (
	Version   = "0.1.3"
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
	Version  string   `json:"version"`
	Date     string   `json:"date"`
	Title    string   `json:"title"`
	Added    []string `json:"added,omitempty"`
	Changed  []string `json:"changed,omitempty"`
	Security []string `json:"security,omitempty"`
	Fixed    []string `json:"fixed,omitempty"`
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

func String() string {
	if Commit == "" || Commit == "dev" {
		return fmt.Sprintf("%s %s", Product, Version)
	}
	return fmt.Sprintf("%s %s (%s)", Product, Version, Commit)
}

func Changelog() []ChangelogEntry {
	entries := []ChangelogEntry{
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
