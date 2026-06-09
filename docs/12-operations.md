# 运维与安装

详细的服务端和设备端安装步骤请优先参考双语安装指南：[docs/14-installation.md](14-installation.md)。本页保留构建、服务运行和在线更新等运维细节。

## 构建

Windows:

```powershell
$env:GOCACHE='F:\project\GPUFleet\.gocache'
go build -o bin\gpufleet-server.exe .\cmd\gpufleet-server
go build -o bin\gpufleet-agent.exe .\cmd\gpufleet-agent
```

前端：

```powershell
cd web
npm install
npm run build
cd ..
```

`web/dist` 已提交，可直接由服务端托管；只有修改前端源码后才需要重新构建。

Linux Agent 交叉编译示例：

```powershell
$env:GOOS='linux'
$env:GOARCH='amd64'
go build -o bin\gpufleet-agent ./cmd/gpufleet-agent
Remove-Item Env:\GOOS
Remove-Item Env:\GOARCH
```

查看二进制版本：

```powershell
.\bin\gpufleet-server.exe -version
.\bin\gpufleet-agent.exe -version
```

发布构建时建议注入当前提交和构建时间，设置页会通过 `GET /api/v1/version` 展示这些信息：

```powershell
$commit = git rev-parse HEAD
$buildTime = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
go build `
  -ldflags "-X gpufleet/internal/version.Commit=$commit -X gpufleet/internal/version.BuildTime=$buildTime" `
  -o bin\gpufleet-server.exe .\cmd\gpufleet-server
```

## Windows Agent 服务

以管理员 PowerShell 运行：

```powershell
.\scripts\install-agent-windows.ps1 `
  -ServerUrl "https://your-server:8443" `
  -DeviceId "device_xxx" `
  -Secret "replace-with-device-secret"
```

卸载：

```powershell
.\scripts\uninstall-agent-windows.ps1
```

连文件一起删除：

```powershell
.\scripts\uninstall-agent-windows.ps1 -RemoveFiles
```

## Linux Agent 服务

在 Linux 目标机上放置 `bin/gpufleet-agent`、`scripts/install-agent-linux.sh` 和 `scripts/gpufleet-agent.service` 后运行：

```sh
sudo SERVER_URL="https://your-server:8443" \
  DEVICE_ID="device_xxx" \
  SECRET="replace-with-device-secret" \
  sh ./scripts/install-agent-linux.sh
```

卸载：

```sh
sudo sh ./scripts/uninstall-agent-linux.sh
```

删除配置和队列：

```sh
sudo REMOVE_FILES=1 sh ./scripts/uninstall-agent-linux.sh
```

## 服务端运行

```powershell
.\bin\gpufleet-server.exe `
  -addr 0.0.0.0:8080 `
  -data-dir data `
  -min-free-mb 800 `
  -retention-days 30 `
  -web-dir web/dist `
  -repo-dir .
```

首次启动按 `-addr` 指定的端口使用 HTTP。浏览器打开 Web 面板后会先选择界面语言，再进入首次配置引导，设置访问密码、下一次启动端口和可选 HTTPS 证书。上传证书后服务端会自动调度重启，恢复后服务端自身使用 HTTPS；未配置证书时继续使用 HTTP。

`-admin-password` 仍可用于自动化部署预置初始密码，但普通部署建议留空并使用首次配置引导。生产环境也可以在服务端前面放 Caddy/Nginx/Traefik 终止 HTTPS，再反代到 GPUFleet 的 HTTP 监听地址。

`-bootstrap-device-id` 和 `-bootstrap-secret` 只用于自动化预创建一个初始 Agent 设备。普通生产部署建议不要传这两个参数，而是在 Web 面板的“设备”页创建设备；如果曾经删除过引导设备，未显式配置 bootstrap 时服务端重启不会重新创建它。

界面语言支持 `zh-CN` 和 `en-US`，保存在服务端数据目录的 `metadata.json` 中。登录后可在设置页单独修改语言，语言切换不需要重启服务。

设置页还提供磁盘预留空间、自动更新开关、手动重启服务和访客功能开关。磁盘预留空间保存后立即影响服务端写入保护；自动更新默认开启并每 30 分钟检查一次上游；手动重启会调度当前服务端进程重启，Web 面板全屏等待恢复并提示重启成功；访客功能开启后登录页显示访客入口，关闭后访客总览和访客曲线接口都会返回 `403`。

## 服务端在线更新

设置页的“在线更新”用于检查、构建、拉取并自动重启服务端自身 Git 仓库更新。自动更新默认开启，每 30 分钟检查一次 upstream；发现可 fast-forward 更新时会自动构建、拉取并重启。服务端必须从 Git checkout 启动，并且当前分支需要配置 upstream，例如 `main` 跟踪 `origin/main`。

运行参数：

```powershell
.\bin\gpufleet-server.exe `
  -addr 0.0.0.0:8080 `
  -data-dir data `
  -web-dir web/dist `
  -repo-dir F:\project\GPUFleet
```

规则：

- `-repo-dir` 默认为当前工作目录，也可用 `GPUFLEET_REPO_DIR` 指定。
- 检查更新会执行固定 Git 状态检查和 `git fetch --quiet --prune`。前端会缓存更新状态 1 小时，打开设置页时优先显示缓存；点击检查更新可立即刷新。
- 自动更新开关保存在数据目录的 `metadata.json` 中；旧 metadata 没有该字段时按开启处理。
- 设置页可保存在线更新代理地址，Git 和 Go 构建过程会复用该代理环境。
- 点击“更新”会先显示全屏确认弹窗，再检查 `git`、`go`、Windows 的 `powershell.exe` 或 Linux 的 `/bin/sh`、服务端源码入口和当前可执行文件目录写入权限。
- 服务端会在临时 Git worktree 中构建远端提交，构建成功后才执行 `git pull --ff-only`。
- 工作区存在未提交改动、没有 upstream、本地超前或与上游分叉时会阻止更新。
- 拉取完成后会生成平台重启器，等待旧进程退出后替换当前服务端二进制，并按原启动参数拉起新进程。重启日志写入当前二进制目录的 `gpufleet-update-restart.log`。
- 自动更新成功后会在服务端保存一条待展示通知。下一次管理员访问时，面板通过 `/api/v1/admin/update/notice` 读取并清除该通知，显示更新时间和更新内容；如果版本号未变化，只展示新旧 `CHANGELOG.md` 顶部同版本条目中新增或变化的行，完全一致时显示“无更新说明”。
- Web 面板会在更新、证书启用或手动重启时显示全屏背景模糊进度。重启阶段进度停在 99%，服务恢复后刷新当前页面，并弹出需要确认的完成提示。

## 访客模式

访客模式默认关闭。登录后可在设置页左下区域开启“访客功能”，开启后登录页显示访客入口。

访客访问范围：

- URL 为 `/guest`。
- 只展示脱敏总览、设备名称/在线状态和 GPU 卡片曲线。
- 不展示 GPU 进程、24 小时统计、管理入口、真实设备 ID、主机名、Agent 版本、驱动版本、GPU UUID 和 VBIOS。
- 访客曲线使用独立的访客接口，关闭访客功能后不可继续读取。

设置页“访客记录”弹窗会显示最近 100 次访问，包括远端地址、User-Agent、语言、平台、屏幕、时区和浏览器指纹摘要。

## 设备注册和密钥轮换

推荐从 Web 面板的“设备”页注册新设备。注册后页面会显示一次性设备密钥，把该密钥写入目标机器的 Agent 本地配置或安装脚本参数：

```powershell
.\scripts\install-agent-windows.ps1 `
  -ServerUrl "https://your-server:8443" `
  -DeviceId "device_20260602120000" `
  -Secret "replace-with-one-time-secret"
```

轮换密钥后，旧密钥立即失效。服务端只更新认证记录，不会远程修改客户端配置；需要设备管理员在客户端本机同步更新 Agent 的密钥。

禁用设备会让服务端拒绝该设备继续上报，适合设备退役、密钥泄露或临时阻断接入。

## 安全注意

- Agent 服务只主动访问服务端，不监听端口。
- 服务端 API 不提供命令下发、配置下发或远程执行。
- 在线更新只影响服务端自身 Git 工作区，不会升级或修改客户端 Agent。
- `agent.env` 和 Windows 服务参数包含设备密钥，应限制读取权限。
- Agent 本地队列只缓存 GPU 指标样本，不回放进程快照。
