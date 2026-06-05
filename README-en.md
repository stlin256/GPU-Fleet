<h1>
  <img src="web/public/brand/gpufleet-logo.svg" alt="GPUFleet logo" width="40" height="40" align="absmiddle">
  GPUFleet
</h1>

[![DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/stlin256/GPU-Fleet/8-glossary)

GPUFleet is a lightweight NVIDIA GPU fleet monitoring system. It combines a public server and Windows/Linux Agents to collect GPU runtime data from machines across different networks, then presents current status, history, statistics, device management, database download, and operations controls in a responsive web dashboard.

Chinese documentation: [README.md](README.md)

Current version: `0.1.7`<br>
Author: `stlin256`<br>
Repository: `https://github.com/stlin256/GPU-Fleet`

## Goals

- Agents only perform local read-only collection and actively report to the public server.
- The server only receives, verifies, stores, summarizes, and displays data; it never sends commands or configuration to clients.
- The default deployment stays simple: one Go server binary, one Go Agent binary, gzip JSONL metric segments, and JSON metadata.
- Storage has retention cleanup and a disk guard with an `800MiB` default free-space reserve.
- The web dashboard prioritizes multi-host, multi-GPU cards with historical charts, offline masks, same-device border colors, hover readings, dark/light themes, and mobile support.
- First startup begins with language selection. Simplified Chinese and English are supported; language can later be changed from Settings and is stored in server metadata.

## Current Capabilities

| Module | Status | Notes |
| --- | --- | --- |
| Server | Implemented | HTTP/HTTPS, HMAC Agent ingestion, Web API, static dashboard hosting, built-in fallback dashboard |
| Agent | Implemented | Windows/Linux, read-only `nvidia-smi` collection, offline queue |
| Web dashboard | Implemented | React, Vite, TypeScript, TanStack Query, ECharts, lucide-react |
| i18n | Implemented | First-start language selection, Settings language switch, server persistence, extensible dictionary, Chinese/English |
| Device management | Implemented | Create, one-time secret, rename, enable/disable, delete, rotate secret, confirmation dialogs |
| Storage | Implemented | gzip JSONL metrics, JSON metadata, retention cleanup, database download |
| Security | Implemented | HMAC signatures, nonce replay protection, login rate limit, progressive lockout, 30-day sessions |
| Online update | Implemented | Git upstream check, dependency preflight, remote build, fast-forward pull, automatic server restart |

## First Startup

The first browser visit opens a setup flow:

1. Choose the interface language.
2. Set the web access password.
3. Set the next startup port.
4. Optionally upload an HTTPS certificate and private key.

Language changes apply immediately. Port and HTTPS certificate changes require restarting the current server process.

## Run Server

```powershell
.\bin\gpufleet-server.exe `
  -addr 0.0.0.0:9008 `
  -data-dir data `
  -min-free-mb 800 `
  -retention-days 30 `
  -web-dir web/dist `
  -repo-dir .
```

## Register Agents

Create devices from the dashboard Devices page, copy each one-time secret, then run the Agent on the target machine:

```sh
./bin/gpufleet-agent \
  -server-url https://your-server:9008 \
  -device-id device_xxx \
  -secret replace-with-device-secret \
  -processes
```

## Documentation

- Product: [docs/01-product.md](docs/01-product.md)
- Architecture: [docs/02-architecture.md](docs/02-architecture.md)
- Security: [docs/03-security.md](docs/03-security.md)
- API: [docs/06-api-contract.md](docs/06-api-contract.md)
- Frontend: [docs/07-frontend.md](docs/07-frontend.md)
- Operations: [docs/12-operations.md](docs/12-operations.md)
- i18n: [docs/13-i18n.md](docs/13-i18n.md)
