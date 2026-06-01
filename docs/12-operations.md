# 运维与安装

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
  -admin-password "change-me" `
  -bootstrap-device-id "device_xxx" `
  -bootstrap-secret "replace-with-device-secret" `
  -min-free-mb 800 `
  -retention-days 30 `
  -web-dir web/dist
```

生产环境建议在服务端前面放 Caddy/Nginx/Traefik 终止 HTTPS，再反代到 `127.0.0.1:8080`。

## 安全注意

- Agent 服务只主动访问服务端，不监听端口。
- 服务端 API 不提供命令下发、配置下发或远程执行。
- `agent.env` 和 Windows 服务参数包含设备密钥，应限制读取权限。
- Agent 本地队列只缓存 GPU 指标样本，不回放进程快照。
