<h1>
  <img src="web/public/brand/gpufleet-logo.svg" alt="GPUFleet logo" width="40" height="40" align="absmiddle">
  GPUFleet
</h1>

[![DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/stlin256/GPU-Fleet/8-glossary)

GPUFleet is a lightweight NVIDIA GPU fleet monitoring system. It combines a public server and Windows/Linux Agents to collect GPU runtime data from machines across different networks, then presents current status, history, statistics, device management, database download, and operations controls in a responsive web dashboard.

Chinese documentation: [README.md](README.md)

Installation guide: [docs/14-installation.md](docs/14-installation.md)

Current version: `0.1.9`<br>
Author: `stlin256`<br>
Repository: `https://github.com/stlin256/GPU-Fleet`

## Goals

- Agents only perform local read-only collection and actively report to the public server.
- The server only receives, verifies, stores, summarizes, and displays data; it never sends commands or configuration to clients.
- The default deployment stays simple: one Go server binary, one Go Agent binary, gzip JSONL metric segments, and JSON metadata.
- Storage has retention cleanup and a disk guard with an `800MiB` default free-space reserve.
- The web dashboard prioritizes multi-host, multi-GPU cards with historical charts, offline masks, same-device border colors, hover readings, dark/light themes, and mobile support.
- First startup begins with language selection. Simplified Chinese and English are supported; language can later be changed from Settings and is stored in server metadata.
- Optional guest access exposes only a sanitized `/guest` overview and guest-only GPU series endpoints. Guests cannot access processes, 24-hour stats, real device IDs, hostnames, Agent metadata, driver versions, GPU UUIDs, or admin APIs.

## Current Capabilities

| Module | Status | Notes |
| --- | --- | --- |
| Server | Implemented | HTTP/HTTPS, HMAC Agent ingestion, Web API, static dashboard hosting, built-in fallback dashboard |
| Agent | Implemented | Windows/Linux, read-only `nvidia-smi` collection, offline queue |
| Web dashboard | Implemented | React, Vite, TypeScript, TanStack Query, ECharts, lucide-react |
| i18n | Implemented | First-start language selection, Settings language switch, server persistence, extensible dictionary, Chinese/English |
| Device management | Implemented | Create, one-time secret, rename, enable/disable, delete, rotate secret, confirmation dialogs |
| Storage | Implemented | gzip JSONL metrics, in-memory rollups/indexes, schema-versioned JSON metadata, retention cleanup, database download |
| Security | Implemented | HMAC signatures, atomic nonce replay protection, Origin/Referer checks for management writes, login rate limit, progressive lockout, 30-day sessions |
| Guest access | Implemented | Login-page guest entry, sanitized overview, guest-only series API, visit records with browser fingerprint summaries |
| Release info | Implemented | `-version`, `/api/v1/version`, current release summary, full changelog dialog, `CHANGELOG.md` |
| Online update | Implemented | Default-on 30-minute automatic checks, manual checks, 1-hour status cache, proxy setting, confirmation dialog, dependency preflight, remote build, fast-forward pull, automatic restart, and completion notices |

## Product Screenshots

![GPUFleet Overview Dashboard](imgs/1-en.png)

![GPUFleet Device Management](imgs/2-en.png)

![GPUFleet GPU Monitoring](imgs/3-en.png)

![GPUFleet Service Settings](imgs/4-en.png)

## First Startup

The first browser visit opens a setup flow:

1. Choose the interface language.
2. Set the web access password.
3. Set the next startup port.
4. Optionally upload an HTTPS certificate and private key.

Language changes apply immediately. Port and HTTPS certificate changes require restarting the current server process.

## Dashboard

The authenticated dashboard has Overview, GPU, Devices, and Settings views. Overview and GPU cards include compact sparklines and 24-hour expandable GPU charts. Settings includes service status, password, port, language, HTTPS certificates, database download, disk reserve, automatic/manual online update, manual service restart, guest access, setup wizard, repository attribution, release information, and the changelog dialog.

The guest dashboard at `/guest` is intentionally smaller: it shows a sanitized overview and GPU chart cards only. It hides GPU processes, 24-hour statistics, management controls, real device identifiers, host metadata, and internal GPU identifiers.

All confirmation, progress, guest records, changelog, update, restart, and fallback-dashboard dialogs use full-viewport blurred backdrops so they are not constrained by the current tab or panel layout.

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

## Server Operations

Online update operates only on the server Git checkout configured by `-repo-dir` or `GPUFLEET_REPO_DIR`. Automatic checks are enabled by default and run every 30 minutes; when a fast-forwardable update exists, the server builds the remote commit in a temporary worktree, fast-forwards only after a successful build, keeps a `.bak` copy of the previous binary, replaces the running server binary, and restarts. The update panel caches status for one hour, supports an update proxy URL, and still allows manual checks and manual apply.

After an automatic update completes, the next admin visit shows a completion dialog with update time and notes. If the version did not change, the dialog shows only new or changed `CHANGELOG.md` lines since the previous checkout; if the changelog is identical, it shows “No update notes.”

Settings also provides a manual service restart button. HTTPS certificate upload schedules an automatic restart; after recovery the page refreshes and shows a completion dialog that must be acknowledged.

## Documentation

- Installation: [docs/14-installation.md](docs/14-installation.md)
- Product: [docs/01-product.md](docs/01-product.md)
- Architecture: [docs/02-architecture.md](docs/02-architecture.md)
- Security: [docs/03-security.md](docs/03-security.md)
- API: [docs/06-api-contract.md](docs/06-api-contract.md)
- Frontend: [docs/07-frontend.md](docs/07-frontend.md)
- Operations: [docs/12-operations.md](docs/12-operations.md)
- i18n: [docs/13-i18n.md](docs/13-i18n.md)
