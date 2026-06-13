# Installation / 安装指南

This page is the operational installation guide for GPUFleet. It covers the public server and device Agents on Linux, Windows, and WSL2.

本文档是 GPUFleet 的实际安装指南，覆盖公网服务端以及 Linux、Windows、WSL2 设备端 Agent。

## Server On Linux / Linux 服务端

### Install From Release Package / 使用发布包安装

Download the matching `gpufleet-server_<version>_linux_<arch>.tar.gz`, extract it, and run the installer as root. Release packages include a prebuilt server binary, so Go is not required on the server.

下载匹配的 `gpufleet-server_<version>_linux_<arch>.tar.gz`，解压后以 root 执行安装脚本。发布包包含预编译服务端二进制，因此服务器不需要安装 Go。

```sh
tar -xzf gpufleet-server_1.0.14_linux_amd64.tar.gz
cd gpufleet-server_1.0.14_linux_amd64

ADDR="0.0.0.0:9008" \
DATA_DIR="/var/lib/gpufleet" \
INSTALL_DIR="/opt/gpufleet" \
sh ./scripts/install-server-linux.sh
```

### Quick Install After Clone / Clone 后一键安装

Run these commands on the Linux server. The script builds `gpufleet-server`, writes a systemd unit, enables auto-start, and starts the service.

在 Linux 服务端执行以下命令。脚本会构建 `gpufleet-server`、写入 systemd 服务、开启开机自启动并启动服务。

```sh
git clone https://github.com/stlin256/GPU-Fleet.git /opt/gpufleet/repo
cd /opt/gpufleet/repo

ADDR="0.0.0.0:9008" \
DATA_DIR="/var/lib/gpufleet" \
INSTALL_DIR="/opt/gpufleet" \
sh ./scripts/install-server-linux.sh
```

Open the dashboard:

打开面板：

```text
http://your-server:9008
```

First startup opens the setup flow: choose language, set the web password, choose the next startup port, and optionally upload HTTPS certificate files.

首次启动会进入配置引导：选择语言、设置 Web 密码、设置下次启动端口，并可选上传 HTTPS 证书文件。

### Script Options / 脚本参数

The installer is configured with environment variables:

安装脚本通过环境变量配置：

| Variable / 变量 | Default / 默认值 | Description / 说明 |
| --- | --- | --- |
| `ADDR` | `0.0.0.0:9008` | Listen address / 监听地址 |
| `REPO_DIR` | current directory / 当前目录 | Git checkout used by online update / 在线更新使用的 Git 工作区 |
| `INSTALL_DIR` | `/opt/gpufleet` | Binary directory / 二进制安装目录 |
| `BIN_PATH` | `$INSTALL_DIR/gpufleet-server` | Server binary path / 服务端二进制路径 |
| `DATA_DIR` | `/var/lib/gpufleet` | Runtime data directory / 运行数据目录 |
| `WEB_DIR` | `$REPO_DIR/web/dist` | Dashboard static files / Web 静态文件目录 |
| `MIN_FREE_MB` | `800` | Minimum reserved free disk space / 最小预留磁盘空间 |
| `RETENTION_DAYS` | `30` | Metric retention days / 指标保留天数 |
| `SERVICE_NAME` | `gpufleet-server` | systemd service name / systemd 服务名 |
| `ADMIN_PASSWORD` | empty / 空 | Optional initial admin password / 可选初始管理员密码 |

Example with HTTPS terminating directly in GPUFleet:

GPUFleet 自身启用 HTTPS 的示例：

```sh
cd /opt/gpufleet/repo
ADDR="0.0.0.0:9008" sh ./scripts/install-server-linux.sh
```

Then upload the full certificate chain and private key from Settings. The server schedules a restart after certificate upload and the dashboard waits for recovery before showing the completion notice.

然后在设置页上传完整证书链和私钥。证书上传后服务端会调度重启，面板会等待服务恢复并显示完成提示。

After setup, Settings can also download the database or diagnostics package, adjust the disk reserve, toggle the default-on 30-minute automatic update checks, configure the online update proxy, run a manual service restart, enable guest access, and open guest visit records.

配置完成后，设置页还可以下载数据库或诊断包、调整磁盘预留空间、切换默认开启的 30 分钟自动更新检查、配置在线更新代理、手动重启服务、开启访客访问并查看访客记录。

### Service Commands / 服务命令

```sh
systemctl status gpufleet-server --no-pager -l
journalctl -u gpufleet-server -f
systemctl restart gpufleet-server
systemctl stop gpufleet-server
```

The service environment file is:

服务环境变量文件：

```text
/etc/gpufleet/server.env
```

After editing it, reload and restart:

修改后重新加载并重启：

```sh
systemctl daemon-reload
systemctl restart gpufleet-server
```

### Backup And Restore / 备份与恢复

Create a live backup without stopping the dashboard:

不中断面板创建热备份：

```sh
DATA_DIR="/var/lib/gpufleet" \
BACKUP_DIR="/var/backups/gpufleet" \
sh ./scripts/backup-server-linux.sh
```

For a cold backup, stop the service during the archive step:

冷备份会在归档期间停止服务：

```sh
STOP_SERVICE=1 sh ./scripts/backup-server-linux.sh
```

Restore requires an explicit confirmation flag. The script stops the service, moves the existing data directory to a timestamped rollback path, restores the archive, and starts the service again.

恢复必须显式确认。脚本会停止服务，把现有数据目录移动到带时间戳的回滚路径，再恢复归档并重新启动服务。

```sh
CONFIRM_RESTORE=1 \
BACKUP_FILE="/var/backups/gpufleet/gpufleet-data-20260609-120000.tar.gz" \
sh ./scripts/restore-server-linux.sh
```

Backups include server metadata, metrics, sessions, device records, and certificate files under the data directory. Store backup files as sensitive material.

备份包含数据目录中的服务端元数据、指标、会话、设备记录和证书文件，应按敏感资料保存。

### Manual Upgrade Of Older Deployments / 旧部署手动升级

If an older deployment was started manually, clone or update the repository, then run the installer once. It will replace the binary, write the service, and make future online updates possible.

如果旧版本是手动启动的，先 clone 或更新仓库，再运行一次安装脚本。它会替换二进制、写入服务，并让后续在线更新可用。

Online update expects the service checkout to track the official `github.com/stlin256/gpu-fleet` upstream. If a network remote points to another repository, update apply is blocked until the deployment is corrected.

在线更新要求服务端工作区跟踪官方 `github.com/stlin256/gpu-fleet` upstream。如果网络远端指向其他仓库，应用更新会被阻止，直到部署状态被修正。

```sh
cd /opt/gpufleet/repo
git pull --ff-only

ADDR="0.0.0.0:9008" \
DATA_DIR="/var/lib/gpufleet" \
INSTALL_DIR="/opt/gpufleet" \
sh ./scripts/install-server-linux.sh
```

Verify the running binary:

验证当前二进制：

```sh
/opt/gpufleet/gpufleet-server -version
systemctl status gpufleet-server --no-pager -l
```

## Device Agent On Linux / Linux 设备端 Agent

Use the Devices page to create a device, then copy the generated `device_id` and secret. Run the installer on the target Linux machine:

先在设备页创建设备，然后复制生成的 `device_id` 和密钥。在目标 Linux 设备上运行：

```sh
cd /tmp/gpufleet

SERVER_URL="https://your-server:9008" \
DEVICE_ID="device_xxx" \
SECRET="replace-with-one-time-secret" \
INTERVAL="10" \
CONFIG_INTERVAL="3600" \
UPDATE_CHECK_INTERVAL="1800" \
QUEUE_MAX_MB="128" \
sh ./scripts/install-agent-linux.sh
```

The Linux Agent installer copies `./bin/gpufleet-agent` to `/usr/local/bin/gpufleet-agent`, writes `/etc/gpufleet/agent.env`, enables `gpufleet-agent.service`, and starts it.

Linux Agent 安装脚本会把 `./bin/gpufleet-agent` 复制到 `/usr/local/bin/gpufleet-agent`，写入 `/etc/gpufleet/agent.env`，启用并启动 `gpufleet-agent.service`。

Check logs:

查看日志：

```sh
systemctl status gpufleet-agent --no-pager -l
journalctl -u gpufleet-agent -f
```

Uninstall:

卸载：

```sh
sh ./scripts/uninstall-agent-linux.sh
REMOVE_FILES=1 sh ./scripts/uninstall-agent-linux.sh
```

## Device Agent On Windows / Windows 设备端 Agent

Use an elevated PowerShell window. Prefer a release package that contains `bin\gpufleet-agent.exe` and `scripts\install-agent-windows.ps1`; Windows devices do not need Go when using a release package.

使用管理员 PowerShell。推荐使用包含 `bin\gpufleet-agent.exe` 和 `scripts\install-agent-windows.ps1` 的发布包；使用发布包时，Windows 设备不需要安装 Go。

```powershell
.\scripts\install-agent-windows.ps1 `
  -ServerUrl "https://your-server:9008" `
  -DeviceId "device_xxx" `
  -Secret "replace-with-one-time-secret" `
  -IntervalSeconds 10 `
  -ConfigIntervalSeconds 3600 `
  -UpdateCheckIntervalSeconds 1800 `
  -QueueMaxMB 128
```

The Windows installer validates the Agent version, optionally runs a one-shot upload preflight, writes credentials to `C:\ProgramData\GPUFleet\agent.env` with restricted ACLs, and creates an automatic scheduled task named `GPUFleetAgent`. Logs are written to `C:\ProgramData\GPUFleet\logs\agent.log`. Older Windows Service installs with the same name are stopped and removed during installation.

Windows 安装脚本会校验 Agent 版本，可选执行一次性上报预检，把凭据写入带受限 ACL 的 `C:\ProgramData\GPUFleet\agent.env`，并创建名为 `GPUFleetAgent` 的开机自启计划任务。日志写入 `C:\ProgramData\GPUFleet\logs\agent.log`。同名旧 Windows Service 会在安装时停止并删除。

Task and log commands:

计划任务与日志命令：

```powershell
Get-ScheduledTask GPUFleetAgent
Start-ScheduledTask GPUFleetAgent
Stop-ScheduledTask GPUFleetAgent
Get-Content "C:\ProgramData\GPUFleet\logs\agent.log" -Tail 100
```

When installation fails during the preflight, rerun the foreground Agent command to expose configuration, authentication, TLS, or connectivity errors. Use `-SkipOnceCheck` only when the device cannot reach the server during installation but should still be installed for later network recovery.

如果安装在预检阶段失败，前台运行 Agent 以暴露配置、认证、TLS 或连通性错误。只有当设备安装时暂时无法访问服务端、但需要等网络恢复后自动上线时，才使用 `-SkipOnceCheck`。

```powershell
.\bin\gpufleet-agent.exe `
  -server-url "https://your-server:9008" `
  -device-id "device_xxx" `
  -secret "replace-with-one-time-secret" `
  -once
```

Skip preflight when needed:

需要跳过预检时：

```powershell
.\scripts\install-agent-windows.ps1 `
  -ServerUrl "https://your-server:9008" `
  -DeviceId "device_xxx" `
  -Secret "replace-with-one-time-secret" `
  -SkipOnceCheck
```

Uninstall:

卸载：

```powershell
.\scripts\uninstall-agent-windows.ps1
.\scripts\uninstall-agent-windows.ps1 -RemoveFiles
```

## WSL2 Notes / WSL2 说明

For Windows machines, installing the Windows Agent is usually preferred. It starts with Windows and reports the Windows host directly.

Windows 机器通常建议安装 Windows Agent。它会随 Windows 自启动，并直接上报 Windows 主机。

Install inside WSL2 only when the GPU workload runs inside WSL2 and `nvidia-smi` works there:

只有当 GPU 负载运行在 WSL2 内，并且 WSL2 里 `nvidia-smi` 可用时，才建议把 Agent 安装到 WSL2：

```sh
nvidia-smi
SERVER_URL="https://your-server:9008" \
DEVICE_ID="device_xxx" \
SECRET="replace-with-one-time-secret" \
sh ./scripts/install-agent-linux.sh
```

WSL2 systemd must be enabled for the Linux installer to manage services. If systemd is unavailable, run the Agent manually or install the Windows Agent instead.

WSL2 需要启用 systemd，Linux 安装脚本才能管理服务。如果没有 systemd，请手动运行 Agent，或改用 Windows Agent。

## Release Artifacts / 发布包

Official release builds are generated by `scripts/build-release.ps1` and the GitHub Release workflow. The default `full` release matrix builds separate Server and Agent packages for Windows, Linux, macOS, and FreeBSD across Go-supported common architectures. Windows/Linux are the supported Agent operating systems for NVIDIA GPU collection; macOS and FreeBSD Agent packages are provided for completeness and diagnostics where `nvidia-smi` is available.

官方发布包由 `scripts/build-release.ps1` 和 GitHub Release 工作流生成。默认 `full` 发布矩阵会为 Windows、Linux、macOS 和 FreeBSD 的 Go 常见支持架构分别构建 Server 与 Agent 包。NVIDIA GPU 采集实际支持的 Agent 操作系统仍以 Windows/Linux 为主；macOS 和 FreeBSD Agent 包主要用于完整性和具备 `nvidia-smi` 环境时的诊断。

```text
gpufleet-server_<version>_windows_amd64.zip
gpufleet-agent_<version>_windows_amd64.zip
gpufleet-server_<version>_windows_arm64.zip
gpufleet-agent_<version>_windows_arm64.zip
gpufleet-server_<version>_linux_amd64.tar.gz
gpufleet-agent_<version>_linux_amd64.tar.gz
gpufleet-server_<version>_linux_arm64.tar.gz
gpufleet-agent_<version>_linux_arm64.tar.gz
gpufleet-server_<version>_darwin_arm64.tar.gz
gpufleet-agent_<version>_darwin_arm64.tar.gz
gpufleet-server_<version>_freebsd_amd64.tar.gz
gpufleet-agent_<version>_freebsd_amd64.tar.gz
gpufleet_<version>_checksums.txt
```

The full matrix also includes additional Go-supported architectures such as Windows 386, Linux 386/armv5/armv6/armv7/loong64/mips*/ppc64*/riscv64/s390x, Darwin amd64, and FreeBSD 386/armv6/armv7/arm64/riscv64 when they compile.

完整矩阵还会在可编译时包含 Windows 386、Linux 386/armv5/armv6/armv7/loong64/mips*/ppc64*/riscv64/s390x、Darwin amd64、FreeBSD 386/armv6/armv7/arm64/riscv64 等 Go 支持架构。

Windows Agent installation from a release package:

Windows Agent 发布包安装：

```powershell
Expand-Archive .\gpufleet-agent_<version>_windows_amd64.zip -DestinationPath C:\GPUFleet-agent
cd C:\GPUFleet-agent
.\scripts\install-agent-windows.ps1 `
  -ServerUrl "https://your-server:9008" `
  -DeviceId "device_xxx" `
  -Secret "replace-with-one-time-secret"
```

Build all release artifacts locally:

本地构建完整发布包：

```powershell
.\scripts\build-release.ps1
```

Build only the smaller core matrix:

只构建较小的核心矩阵：

```powershell
.\scripts\build-release.ps1 -TargetSet core
```

Build explicit targets:

只构建指定目标：

```powershell
.\scripts\build-release.ps1 -Targets windows/amd64,linux/amd64,linux/arm64
```

Publish a GitHub Release by pushing a version tag:

推送版本标签发布 GitHub Release：

```sh
git tag v1.0.14
git push origin v1.0.14
```

## Build Requirements / 构建要求

Server installer requirements:

服务端安装脚本要求：

- Linux with systemd / 带 systemd 的 Linux
- Release package install: no Go required / 发布包安装：不需要 Go
- Source install: `git` and Go matching `go.mod` / 源码安装：需要 `git` 和与 `go.mod` 匹配的 Go
- Source install needs committed `web/dist` files, or Node.js + npm to rebuild the frontend / 源码安装需要已提交的 `web/dist`，或 Node.js + npm 用于重建前端

Agent requirements:

设备端 Agent 要求：

- NVIDIA driver / NVIDIA 驱动
- `nvidia-smi` available in `PATH` / `PATH` 中可执行 `nvidia-smi`
- outbound network access to the server / 可主动访问服务端

## Connectivity Check / 连通性检查

From a device:

从设备端检查：

```sh
curl -I https://your-server:9008
nvidia-smi
```

Expected Agent logs after successful upload:

成功上报后 Agent 日志中不应持续出现：

```text
upload failed
```

If the server uses HTTPS, upload the full certificate chain, not only the leaf certificate.

如果服务端使用 HTTPS，请上传完整证书链，而不是只上传站点证书。
