package version

import "fmt"

var (
	Version   = "0.1.1"
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
