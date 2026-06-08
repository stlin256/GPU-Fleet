package server

const dashboardHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>GPUFleet</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f3f5f7;
      --bg-strong: #e8edf1;
      --panel: #ffffff;
      --panel-soft: #f9fbfc;
      --text: #16212c;
      --muted: #687789;
      --line: #d9e0e7;
      --line-strong: #c3ccd6;
      --accent: #146c78;
      --accent-strong: #0f5660;
      --accent-soft: #dff3f2;
      --good: #198754;
      --good-soft: #e0f4e9;
      --warn: #b26a00;
      --warn-soft: #fff0d5;
      --bad: #c54040;
      --bad-soft: #fae2e2;
      --offline: #7b8794;
      --offline-soft: #e7ebef;
      --shadow: 0 14px 34px rgba(30, 42, 54, .08);
      --shadow-soft: 0 8px 22px rgba(30, 42, 54, .06);
    }
    :root[data-theme="dark"] {
      color-scheme: dark;
      --bg: #0e1215;
      --bg-strong: #171d22;
      --panel: #171c21;
      --panel-soft: #1d242b;
      --text: #eef2f4;
      --muted: #9aa8b5;
      --line: #2c3741;
      --line-strong: #40505d;
      --accent: #4db6ac;
      --accent-strong: #6ed1c8;
      --accent-soft: #183b3b;
      --good: #4fc17b;
      --good-soft: #153725;
      --warn: #e0a23a;
      --warn-soft: #3f2b10;
      --bad: #ee6b6b;
      --bad-soft: #421d20;
      --offline: #9aa5af;
      --offline-soft: #26303a;
      --shadow: 0 18px 42px rgba(0, 0, 0, .34);
      --shadow-soft: 0 12px 28px rgba(0, 0, 0, .24);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: linear-gradient(180deg, var(--bg-strong) 0, var(--bg) 320px), var(--bg);
      color: var(--text);
      letter-spacing: 0;
    }
    button, input { font: inherit; }
    button { cursor: pointer; }
    h1, h2, h3, p { margin: 0; }
    h1 { font-size: 30px; line-height: 1.1; }
    h2 { font-size: 16px; }
    h3 { font-size: 14px; }
    p { color: var(--muted); font-size: 13px; margin-top: 5px; }
    .hidden { display: none !important; }
    .login-shell {
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 24px;
    }
    .login-panel, .panel, .metric {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      box-shadow: var(--shadow);
    }
    .login-panel {
      width: min(420px, 100%);
      padding: 24px;
    }
    .login-head, .top-actions {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
    }
    .brand {
      display: flex;
      align-items: center;
      gap: 10px;
      font-size: 20px;
      font-weight: 800;
      margin-bottom: 26px;
    }
    .login-head .brand { margin-bottom: 0; }
    .brand-mark {
      width: 30px;
      height: 30px;
      border-radius: 8px;
      display: block;
      overflow: hidden;
      box-shadow: var(--shadow-soft);
    }
    .brand-mark svg {
      width: 100%;
      height: 100%;
      display: block;
    }
    label {
      display: grid;
      gap: 8px;
      color: var(--muted);
      margin-top: 14px;
    }
    input {
      min-height: 42px;
      padding: 10px 12px;
      border-radius: 8px;
      border: 1px solid var(--line);
      background: var(--panel);
      color: var(--text);
    }
    .primary, .secondary, .icon-button, nav button {
      border: 1px solid var(--line);
      background: var(--panel);
      color: var(--text);
      min-height: 40px;
      border-radius: 8px;
      transition: transform .16s ease, border-color .16s ease, background .16s ease, box-shadow .16s ease;
    }
    button:hover { transform: translateY(-1px); border-color: var(--line-strong); }
    button:disabled { opacity: .65; cursor: default; transform: none; }
    .primary {
      width: 100%;
      display: flex;
      justify-content: center;
      align-items: center;
      gap: 8px;
      margin-top: 18px;
      background: var(--accent);
      border-color: var(--accent);
      color: white;
      box-shadow: 0 10px 22px rgba(20, 108, 120, .22);
    }
    .primary.narrow {
      width: auto;
      min-width: 112px;
      margin-top: 0;
      padding: 0 14px;
      align-self: end;
    }
    .secondary {
      min-height: 36px;
      display: inline-flex;
      align-items: center;
      justify-content: center;
      gap: 7px;
      padding: 8px 10px;
    }
    .icon-button {
      width: 42px;
      display: grid;
      place-items: center;
    }
    .app {
      min-height: 100vh;
      display: grid;
      grid-template-columns: 232px 1fr;
    }
    .sidebar {
      background: rgba(255, 255, 255, .78);
      border-right: 1px solid var(--line);
      padding: 24px 16px;
      position: sticky;
      top: 0;
      height: 100vh;
      backdrop-filter: blur(16px);
    }
    :root[data-theme="dark"] .sidebar { background: rgba(23, 28, 33, .82); }
    nav {
      display: grid;
      gap: 8px;
    }
    nav button {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 10px 12px;
      text-align: left;
    }
    nav button.active {
      background: var(--accent);
      border-color: var(--accent);
      color: white;
      box-shadow: 0 10px 22px rgba(20, 108, 120, .22);
    }
    .content {
      padding: 24px;
      display: grid;
      gap: 18px;
      align-content: start;
    }
    .topbar {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 14px;
    }
    .panel { padding: 16px; }
    .panel-head {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 10px;
      margin-bottom: 10px;
    }
    .pill, .panel-head span {
      border: 1px solid var(--line);
      border-radius: 999px;
      padding: 4px 8px;
      color: var(--muted);
      font-size: 12px;
      white-space: nowrap;
    }
    .pill.online, .pill.good { color: var(--good); border-color: var(--good); background: var(--good-soft); }
    .pill.warning, .pill.warn { color: var(--warn); border-color: var(--warn); background: var(--warn-soft); }
    .pill.critical, .pill.bad { color: var(--bad); border-color: var(--bad); background: var(--bad-soft); }
    .pill.offline, .pill.disabled { color: var(--offline); border-color: var(--offline); background: var(--offline-soft); }
    .banner {
      border-radius: 8px;
      padding: 10px 12px;
      border: 1px solid var(--line);
      background: var(--panel);
    }
    .banner.danger { color: var(--bad); border-color: var(--bad); }
    .notice { color: var(--good); }
    .error { color: var(--bad); }
    .fleet-command {
      border: 1px solid var(--line);
      border-radius: 8px;
      background: linear-gradient(135deg, var(--panel), var(--panel-soft));
      box-shadow: var(--shadow);
      display: grid;
      grid-template-columns: minmax(260px, .95fr) minmax(420px, 1.4fr);
      gap: 18px;
      padding: 18px;
      align-items: stretch;
    }
    .fleet-command-copy {
      display: grid;
      align-content: center;
      gap: 8px;
    }
    .fleet-eyebrow {
      width: fit-content;
      border: 1px solid var(--accent);
      border-radius: 999px;
      color: var(--accent-strong);
      background: var(--accent-soft);
      font-size: 12px;
      font-weight: 800;
      padding: 4px 8px;
    }
    .fleet-command h2 { font-size: 32px; line-height: 1.05; }
    .fleet-kpis {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 10px;
    }
    .fleet-kpi {
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel-soft);
      padding: 12px;
      min-width: 0;
    }
    .fleet-kpi span, .setting-stat span {
      display: block;
      color: var(--muted);
      font-size: 12px;
      font-weight: 800;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .fleet-kpi strong {
      display: block;
      margin-top: 6px;
      font-size: 23px;
      line-height: 1.05;
      overflow-wrap: anywhere;
    }
    .fleet-kpi.good strong { color: var(--good); }
    .fleet-kpi.warn strong { color: var(--warn); }
    .fleet-kpi.bad strong { color: var(--bad); }
    .fleet-kpi.accent strong { color: var(--accent-strong); }
    .overview-layout {
      display: grid;
      grid-template-columns: minmax(0, 1fr) minmax(300px, 360px);
      gap: 14px;
      align-items: start;
    }
    .stack, .overview-secondary, .device-admin { display: grid; gap: 14px; }
    .overview-secondary { grid-template-columns: minmax(300px, .75fr) minmax(0, 1.25fr); }
    .fleet-board { overflow: hidden; }
    .fleet-card-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(310px, 1fr));
      gap: 12px;
    }
    .fleet-gpu-card {
      min-width: 0;
      position: relative;
      border: 2px solid var(--device-color, var(--line));
      border-radius: 8px;
      background: linear-gradient(180deg, var(--panel-soft), var(--panel));
      box-shadow: var(--shadow-soft);
      padding: 14px;
      display: grid;
      gap: 12px;
    }
    .fleet-gpu-card.good { box-shadow: var(--shadow-soft), inset 0 3px 0 var(--good); }
    .fleet-gpu-card.warn { box-shadow: var(--shadow-soft), inset 0 3px 0 var(--warn); }
    .fleet-gpu-card.bad { box-shadow: var(--shadow-soft), inset 0 3px 0 var(--bad); }
    .fleet-gpu-card.offline { box-shadow: var(--shadow-soft), inset 0 3px 0 var(--offline); }
    .fleet-gpu-card.offline > :not(.offline-mask) {
      filter: grayscale(.95);
      opacity: .48;
    }
    .offline-mask {
      position: absolute;
      inset: 0;
      z-index: 2;
      display: grid;
      place-items: center;
      border-radius: 8px;
      background: rgba(123, 135, 148, .24);
      color: var(--offline);
      font-size: 22px;
      font-weight: 900;
      pointer-events: none;
    }
    .fleet-card-top {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 10px;
      min-width: 0;
    }
    .fleet-device-cell {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr);
      gap: 10px;
      align-items: center;
      min-width: 0;
    }
    .fleet-device-cell strong, .list-row strong, .table-row strong { overflow-wrap: anywhere; }
    .fleet-device-cell p,
    .trend-head p,
    .setting-stat p,
    .operation-head p {
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .status-dot {
      width: 9px;
      height: 9px;
      border-radius: 999px;
      background: var(--offline);
      box-shadow: 0 0 0 4px var(--offline-soft);
    }
    .status-dot.good { background: var(--good); box-shadow: 0 0 0 4px var(--good-soft); }
    .status-dot.warn { background: var(--warn); box-shadow: 0 0 0 4px var(--warn-soft); }
    .status-dot.bad { background: var(--bad); box-shadow: 0 0 0 4px var(--bad-soft); }
    .status-dot.offline { background: var(--offline); box-shadow: 0 0 0 4px var(--offline-soft); }
    .gpu-card-meta {
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 8px;
    }
    .gpu-card-meta span {
      min-width: 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel-soft);
      color: var(--muted);
      padding: 8px;
      font-size: 12px;
      line-height: 1.25;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .gpu-trend-grid, .gpu-detail-trend-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 8px;
    }
    .gpu-detail-trend-grid { margin-top: 12px; }
    .trend-tile {
      min-width: 0;
      position: relative;
      min-height: 148px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
      padding: 10px;
      display: grid;
      grid-template-rows: auto minmax(58px, 1fr);
      gap: 8px;
    }
    .trend-tile.good { border-color: var(--good); }
    .trend-tile.warn { border-color: var(--warn); }
    .trend-tile.bad { border-color: var(--bad); }
    .trend-head {
      min-width: 0;
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      gap: 10px;
    }
    .trend-head span {
      display: block;
      color: var(--muted);
      font-size: 12px;
      font-weight: 800;
      white-space: nowrap;
    }
    .trend-head strong {
      display: block;
      margin-top: 6px;
      font-size: 16px;
      line-height: 1.2;
      overflow-wrap: anywhere;
    }
    .trend-head p {
      margin-top: 0;
      line-height: 1.25;
      text-align: right;
      max-width: 96px;
    }
    .sparkline-wrap { position: relative; width: 100%; height: 74px; align-self: end; }
    .sparkline { width: 100%; height: 100%; display: block; }
    .spark-grid { fill: none; stroke: var(--line); stroke-width: 1; }
    .spark-area { fill: rgba(20, 108, 120, .16); }
    .spark-line { fill: none; stroke: var(--accent); stroke-width: 3; stroke-linecap: round; stroke-linejoin: round; }
    .spark-cursor { stroke: var(--text); stroke-width: 1.2; stroke-dasharray: 4 3; opacity: .58; }
    .spark-point { fill: var(--panel); stroke: var(--accent); stroke-width: 2.2; }
    .spark-tooltip {
      position: absolute;
      z-index: 4;
      bottom: 100%;
      transform: translate(-50%, -6px);
      min-width: 86px;
      max-width: 132px;
      border: 1px solid var(--line-strong);
      border-radius: 8px;
      background: var(--panel);
      box-shadow: var(--shadow-soft);
      padding: 7px 8px;
      pointer-events: none;
      display: none;
      gap: 2px;
    }
    .spark-tooltip.show { display: grid; }
    .spark-tooltip span, .spark-tooltip small { color: var(--muted); font-size: 11px; line-height: 1.2; }
    .spark-tooltip strong { color: var(--text); font-size: 14px; line-height: 1.15; }
    .metric-spark .spark-tooltip { min-width: 78px; max-width: 112px; padding: 5px 6px; }
    .metric-spark .spark-tooltip span, .metric-spark .spark-tooltip small { font-size: 10px; }
    .metric-spark .spark-tooltip strong { font-size: 12px; line-height: 1.05; }
    .update-note-row { margin-top: -4px; }
    .update-note-row p { min-width: 0; margin: 0; }
    .update-note-row .inline-help { display: inline-grid; width: 22px; height: 22px; margin-left: 6px; vertical-align: -5px; flex: 0 0 auto; }
    .trend-tile.good .spark-line { stroke: var(--good); }
    .trend-tile.warn .spark-line { stroke: var(--warn); }
    .trend-tile.bad .spark-line { stroke: var(--bad); }
    .trend-tile.good .spark-point { stroke: var(--good); }
    .trend-tile.warn .spark-point { stroke: var(--warn); }
    .trend-tile.bad .spark-point { stroke: var(--bad); }
    .metric-grid {
      display: grid;
      grid-template-columns: repeat(5, minmax(140px, 1fr));
      gap: 14px;
    }
    .metric { padding: 14px; }
    .metric p { margin-top: 0; }
    .metric strong {
      display: block;
      margin-top: 8px;
      font-size: 25px;
      overflow-wrap: anywhere;
    }
    .main-grid {
      display: grid;
      grid-template-columns: minmax(0, 1.35fr) minmax(320px, .65fr);
      gap: 14px;
      align-items: start;
    }
    .gpu-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
      gap: 12px;
    }
    .gpu-card {
      border: 2px solid var(--device-color, var(--line));
      border-radius: 8px;
      padding: 14px;
      background: var(--panel-soft);
    }
    .card-title {
      display: flex;
      justify-content: space-between;
      gap: 10px;
    }
    .gpu-detail-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 8px;
      margin-top: 12px;
    }
    .gpu-detail-grid div {
      min-width: 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 8px;
      background: var(--panel);
    }
    .gpu-detail-grid span {
      display: block;
      color: var(--muted);
      font-size: 12px;
      line-height: 1.25;
    }
    .gpu-detail-grid strong {
      display: block;
      margin-top: 4px;
      font-size: 12px;
      line-height: 1.3;
      overflow-wrap: anywhere;
    }
    .list-row, .table-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 10px 0;
      border-bottom: 1px solid var(--line);
    }
    .list-row:last-child, .table-row:last-child { border-bottom: 0; }
    .stats-table { display: grid; }
    .table-row { display: grid; grid-template-columns: minmax(190px, 1fr) repeat(4, minmax(72px, auto)); }
    .empty { padding: 12px 0; }
    .device-form {
      display: grid;
      grid-template-columns: minmax(220px, 1fr) auto;
      gap: 12px;
      align-items: end;
    }
    .device-form label { margin-top: 0; }
    .secret-box {
      margin-top: 14px;
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 12px;
      display: grid;
      grid-template-columns: minmax(150px, .6fr) minmax(180px, 1fr) auto;
      gap: 10px;
      align-items: center;
    }
    .secret-box code {
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 10px;
      overflow-wrap: anywhere;
      background: var(--panel-soft);
    }
    .device-row {
      display: grid;
      grid-template-columns: minmax(220px, 1fr) auto auto;
      gap: 12px;
      align-items: center;
      padding: 12px 0;
      border-bottom: 1px solid var(--line);
    }
    .device-row:last-child { border-bottom: 0; }
    .device-name-cell {
      min-width: 0;
    }
    .device-rename-form {
      display: grid;
      grid-template-columns: minmax(180px, 1fr) auto auto;
      gap: 8px;
      align-items: center;
    }
    .device-rename-form input {
      min-height: 36px;
    }
    .device-rename-form .secondary {
      min-height: 36px;
    }
    .row-actions {
      display: flex;
      gap: 8px;
      flex-wrap: wrap;
      justify-content: flex-end;
    }
    .danger-action {
      color: var(--bad);
      border-color: var(--bad);
    }
    .danger-primary {
      background: var(--bad);
      border-color: var(--bad);
      color: white;
    }
    .modal-backdrop {
      position: fixed;
      inset: 0;
      width: 100vw;
      min-height: 100vh;
      min-height: 100dvh;
      z-index: 80;
      display: grid;
      place-items: center;
      padding: 18px;
      background: rgba(22, 33, 44, .28);
      backdrop-filter: blur(14px);
      isolation: isolate;
    }
    .confirm-dialog {
      width: min(460px, 100%);
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel);
      box-shadow: var(--shadow);
      padding: 18px;
      display: grid;
      gap: 14px;
    }
    .confirm-dialog.warning { border-color: var(--warn); }
    .confirm-dialog.danger { border-color: var(--bad); }
    .confirm-icon {
      width: 42px;
      height: 42px;
      border-radius: 8px;
      display: grid;
      place-items: center;
      color: var(--accent-strong);
      background: var(--accent-soft);
    }
    .confirm-dialog.warning .confirm-icon { color: var(--warn); background: var(--warn-soft); }
    .confirm-dialog.danger .confirm-icon { color: var(--bad); background: var(--bad-soft); }
    .confirm-copy span, .confirm-target span {
      display: block;
      color: var(--muted);
      font-size: 12px;
      font-weight: 800;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .confirm-copy h2 {
      margin-top: 6px;
      font-size: 20px;
    }
    .confirm-copy p { line-height: 1.55; }
    .confirm-target {
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel-soft);
      padding: 10px;
      min-width: 0;
    }
    .confirm-target strong {
      display: block;
      margin-top: 6px;
      overflow-wrap: anywhere;
    }
    .confirm-actions {
      display: flex;
      justify-content: flex-end;
      gap: 10px;
      flex-wrap: wrap;
    }
    #modalRoot {
      position: fixed;
      inset: 0;
      z-index: 80;
      pointer-events: none;
    }
    #modalRoot .modal-backdrop {
      pointer-events: auto;
    }
    .settings-page, .settings-status, .setting-operation { display: grid; gap: 14px; }
    .settings-kpi-grid {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 12px;
    }
    .setting-stat {
      min-width: 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 12px;
      background: var(--panel-soft);
    }
    .setting-stat span { white-space: nowrap; }
    .setting-stat strong {
      display: block;
      margin-top: 7px;
      font-size: 20px;
      line-height: 1.12;
      overflow-wrap: anywhere;
    }
    .settings-workbench {
      display: grid;
      grid-template-columns: minmax(320px, .9fr) minmax(390px, 1.1fr);
      gap: 16px;
      align-items: start;
    }
    .settings-column {
      min-width: 0;
      display: grid;
      gap: 12px;
      align-content: start;
    }
    .settings-section-head {
      min-width: 0;
      display: flex;
      align-items: end;
      justify-content: space-between;
      gap: 12px;
      padding: 2px 2px 0;
    }
    .settings-section-head h2 {
      margin: 0;
      font-size: 14px;
      line-height: 1.2;
      letter-spacing: 0;
    }
    .settings-section-head p {
      margin-top: 4px;
      color: var(--muted);
      font-size: 12px;
      line-height: 1.35;
    }
    .operation-head {
      display: grid;
      grid-template-columns: auto minmax(0, 1fr);
      gap: 10px;
      align-items: center;
    }
    .operation-head > div:nth-child(2) {
      min-width: 0;
    }
    .operation-icon {
      width: 34px;
      height: 34px;
      border-radius: 8px;
      display: grid;
      place-items: center;
      color: var(--accent-strong);
      background: var(--accent-soft);
    }
    .project-card {
      background: linear-gradient(135deg, var(--panel), var(--panel-soft));
    }
    .release-card {
      grid-template-rows: auto auto minmax(0, auto) auto;
    }
    .project-logo {
      padding: 0;
      overflow: hidden;
      background: transparent;
    }
    .project-logo svg {
      width: 100%;
      height: 100%;
      display: block;
    }
    .project-meta {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 10px;
    }
    .project-meta div {
      min-width: 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel-soft);
      padding: 10px;
    }
    .project-meta span {
      display: block;
      color: var(--muted);
      font-size: 12px;
      font-weight: 800;
      white-space: nowrap;
    }
    .project-meta strong {
      display: block;
      margin-top: 6px;
      line-height: 1.15;
      overflow-wrap: anywhere;
    }
    .project-url {
      grid-column: 1 / -1;
    }
    .project-meta a {
      display: block;
      margin-top: 6px;
      color: var(--accent-strong);
      text-decoration: none;
      line-height: 1.2;
      overflow-wrap: anywhere;
    }
    .project-meta a:hover { text-decoration: underline; }
    .changelog-panel {
      min-width: 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel-soft);
      padding: 12px;
      display: grid;
      gap: 10px;
    }
    .changelog-head {
      display: flex;
      align-items: center;
      gap: 7px;
      color: var(--accent-strong);
      font-size: 12px;
      font-weight: 800;
      white-space: nowrap;
    }
    .changelog-entry {
      display: grid;
      gap: 8px;
      min-width: 0;
    }
    .changelog-entry > div:first-child {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 10px;
    }
    .changelog-entry > div:first-child span {
      color: var(--muted);
      font-size: 12px;
      white-space: nowrap;
    }
    .changelog-group {
      display: grid;
      gap: 5px;
    }
    .changelog-group > span {
      color: var(--muted);
      font-size: 12px;
      font-weight: 800;
      white-space: nowrap;
    }
    .changelog-group ul {
      margin: 0;
      padding-left: 18px;
      display: grid;
      gap: 4px;
    }
    .changelog-group li {
      color: var(--text);
      font-size: 12px;
      line-height: 1.35;
    }
    .update-card .operation-head {
      grid-template-columns: auto minmax(0, 1fr) auto;
    }
    .update-compare, .update-meta {
      display: grid;
      gap: 8px;
    }
    .update-compare {
      grid-template-columns: repeat(4, minmax(0, 1fr));
    }
    .update-meta {
      grid-template-columns: minmax(0, 1.25fr) minmax(150px, .75fr);
    }
    .update-compare div, .update-meta div {
      min-width: 0;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: var(--panel-soft);
      padding: 9px;
    }
    .update-compare span, .update-meta span {
      display: block;
      color: var(--muted);
      font-size: 12px;
      font-weight: 800;
      white-space: nowrap;
    }
    .update-compare strong, .update-meta strong {
      display: block;
      margin-top: 6px;
      line-height: 1.15;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .settings-button-row {
      display: flex;
      gap: 8px;
      align-items: center;
      flex-wrap: wrap;
    }
    .update-note {
      margin-top: -4px;
    }
    .action-button {
      width: fit-content;
      text-decoration: none;
    }
    @media (max-width: 1180px) {
      .fleet-command, .overview-layout, .overview-secondary { grid-template-columns: 1fr; }
      .settings-kpi-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    }
    @media (max-width: 980px) {
      .app { grid-template-columns: 1fr; }
      .sidebar {
        position: static;
        height: auto;
        border-right: 0;
        border-bottom: 1px solid var(--line);
        padding: 14px;
      }
      nav { grid-template-columns: repeat(4, minmax(0, 1fr)); }
      nav button { justify-content: center; font-size: 13px; }
      .metric-grid, .main-grid { grid-template-columns: 1fr 1fr; }
      .content { padding: 16px; }
      .table-row { grid-template-columns: 1fr 1fr; align-items: start; }
    }
    @media (max-width: 720px) {
      .topbar { align-items: flex-start; flex-direction: column; }
      .fleet-command { padding: 14px; }
      .fleet-kpis { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      .fleet-card-grid, .gpu-card-meta, .metric-grid, .main-grid { grid-template-columns: 1fr; }
      nav { grid-template-columns: repeat(2, minmax(0, 1fr)); }
      h1 { font-size: 24px; }
      .fleet-command h2 { font-size: 25px; }
      .gpu-detail-grid, .device-form, .device-row, .secret-box, .settings-workbench, .settings-kpi-grid, .project-meta, .update-compare, .update-meta { grid-template-columns: 1fr; }
      .row-actions { justify-content: flex-start; }
    }
    @media (max-width: 430px) {
      .content { padding: 12px; }
      .fleet-kpis { grid-template-columns: 1fr; }
      .gpu-trend-grid, .gpu-detail-trend-grid {
        grid-template-columns: repeat(2, minmax(0, 1fr));
        gap: 6px;
      }
      .trend-tile {
        min-height: 108px;
        grid-template-rows: auto 46px;
        gap: 5px;
        padding: 8px;
      }
      .trend-head { gap: 6px; }
      .trend-head span { font-size: 11px; }
      .trend-head strong {
        margin-top: 4px;
        font-size: 13px;
      }
      .trend-head p {
        max-width: 58px;
        font-size: 10px;
        line-height: 1.2;
      }
      .sparkline-wrap { height: 46px; }
      .spark-line { stroke-width: 2.4; }
      .spark-point { stroke-width: 2; }
      .spark-tooltip {
        min-width: 74px;
        max-width: 106px;
        padding: 6px 7px;
      }
      .spark-tooltip strong { font-size: 12px; }
      .top-actions { width: 100%; }
      .update-card .operation-head {
        grid-template-columns: auto minmax(0, 1fr);
      }
      .update-card .operation-head .pill {
        grid-column: 1 / -1;
        width: fit-content;
      }
    }
  </style>
</head>
<body>
  <div id="login" class="login-shell">
    <form class="login-panel" id="loginForm">
      <div class="login-head">
        <div class="brand"><span class="brand-mark" aria-hidden="true"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256"><defs><linearGradient id="fallbackShellA" x1="28" y1="24" x2="222" y2="228" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#146C78"/><stop offset=".58" stop-color="#198754"/><stop offset="1" stop-color="#B26A00"/></linearGradient><linearGradient id="fallbackChipA" x1="78" y1="73" x2="178" y2="181" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#FFFFFF"/><stop offset="1" stop-color="#DFF3F2"/></linearGradient></defs><rect x="18" y="18" width="220" height="220" rx="46" fill="url(#fallbackShellA)"/><path d="M62 89h34M62 166h34M160 62v34M160 160v34M178 128h32" fill="none" stroke="#E8FBF6" stroke-width="11" stroke-linecap="round" stroke-linejoin="round"/><g fill="#E8FBF6"><circle cx="62" cy="89" r="13"/><circle cx="62" cy="166" r="13"/><circle cx="160" cy="62" r="13"/><circle cx="160" cy="194" r="13"/><circle cx="210" cy="128" r="13"/></g><rect x="84" y="84" width="88" height="88" rx="22" fill="url(#fallbackChipA)"/><rect x="105" y="105" width="46" height="46" rx="12" fill="#146C78"/><path d="M118 130h16c8 0 13-5 13-13s-5-13-13-13h-16v47M121 130h23" fill="none" stroke="#F7FFFC" stroke-width="9" stroke-linecap="round" stroke-linejoin="round"/></svg></span><span>GPUFleet</span></div>
        <button class="icon-button" type="button" id="loginTheme" title="切换主题">◐</button>
      </div>
      <h1>登录面板</h1>
      <p>登录后记住当前设备 30 天</p>
      <label>密码<input name="password" type="password" autocomplete="current-password"></label>
      <button class="primary" type="submit">登录</button>
      <p class="sub error" id="loginError"></p>
    </form>
  </div>
  <div id="app" class="app hidden">
    <aside class="sidebar">
      <div class="brand"><span class="brand-mark" aria-hidden="true"><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256"><defs><linearGradient id="fallbackShellB" x1="28" y1="24" x2="222" y2="228" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#146C78"/><stop offset=".58" stop-color="#198754"/><stop offset="1" stop-color="#B26A00"/></linearGradient><linearGradient id="fallbackChipB" x1="78" y1="73" x2="178" y2="181" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#FFFFFF"/><stop offset="1" stop-color="#DFF3F2"/></linearGradient></defs><rect x="18" y="18" width="220" height="220" rx="46" fill="url(#fallbackShellB)"/><path d="M62 89h34M62 166h34M160 62v34M160 160v34M178 128h32" fill="none" stroke="#E8FBF6" stroke-width="11" stroke-linecap="round" stroke-linejoin="round"/><g fill="#E8FBF6"><circle cx="62" cy="89" r="13"/><circle cx="62" cy="166" r="13"/><circle cx="160" cy="62" r="13"/><circle cx="160" cy="194" r="13"/><circle cx="210" cy="128" r="13"/></g><rect x="84" y="84" width="88" height="88" rx="22" fill="url(#fallbackChipB)"/><rect x="105" y="105" width="46" height="46" rx="12" fill="#146C78"/><path d="M118 130h16c8 0 13-5 13-13s-5-13-13-13h-16v47M121 130h23" fill="none" stroke="#F7FFFC" stroke-width="9" stroke-linecap="round" stroke-linejoin="round"/></svg></span><span>GPUFleet</span></div>
      <nav>
        <button class="active" data-view="overview">总览</button>
        <button data-view="gpus">GPU</button>
        <button data-view="devices">设备</button>
        <button data-view="settings">设置</button>
      </nav>
    </aside>
    <main class="content">
      <header class="topbar">
        <div>
          <h1 id="viewTitle">GPU 资源总览</h1>
          <p id="serverTime">等待服务端数据</p>
        </div>
        <div class="top-actions">
          <button id="themeToggle" class="icon-button" title="切换主题">◐</button>
          <button id="refresh" class="icon-button" title="刷新">↻</button>
          <button id="logout" class="icon-button" title="退出登录">⇥</button>
        </div>
      </header>
      <div id="banner"></div>
      <section id="overviewView" data-page="overview"></section>
      <section id="devicesView" data-page="devices" class="hidden"></section>
      <section id="gpusView" data-page="gpus" class="hidden"></section>
      <section id="settingsView" data-page="settings" class="hidden"></section>
    </main>
  </div>
  <div id="modalRoot"></div>
  <script>
    const fallbackI18n = {
      language: localStorage.getItem('gpufleet-language') || 'zh-CN',
      exact: {
        '登录面板': 'Dashboard login',
        '登录后记住当前设备 30 天': 'This device is remembered for 30 days after login',
        '密码': 'Password',
        '登录': 'Log in',
        '总览': 'Overview',
        '设备': 'Devices',
        '设置': 'Settings',
        'GPU 资源总览': 'GPU Resource Overview',
        '设备管理': 'Device Management',
        'GPU 监控': 'GPU Monitoring',
        '服务设置': 'Service Settings',
        '等待服务端数据': 'Waiting for server data',
        '退出登录': 'Log out',
        '切换主题': 'Toggle theme',
        '刷新': 'Refresh',
        '服务状态': 'Service Status',
        '密码更改': 'Password Change',
        '端口配置': 'Port Configuration',
        'HTTPS 证书': 'HTTPS certificate',
        'HTTPS 已启用': 'HTTPS enabled',
        '数据库下载': 'Database Download',
        '数据库大小': 'Database size',
        '在线更新': 'Online Update',
        '版本与变更': 'Version & Changes',
        '最近变更': 'Latest changes',
        '配置引导': 'Setup wizard',
        '检查更新': 'Check update',
        '拉取并重启': 'Pull and restart',
        '更新代理': 'Update proxy',
        '保存代理': 'Save proxy',
        '查看 Git 原始错误': 'View raw Git error',
        '在线更新失败，请查看详情并检查服务器网络、Git 上游或更新代理配置。': 'Online update failed. View details and check the server network, Git upstream, or update proxy settings.',
        '检查 Git 上游失败': 'Git upstream check failed',
        '当前提交': 'Current commit',
        '远端提交': 'Remote commit',
        '运行版本': 'Running version',
        '仓库版本': 'Repository version',
        '运行提交': 'Running commit',
        '落后': 'Behind',
        '超前': 'Ahead',
        '远端': 'Remote',
        '检查时间': 'Checked at',
        '设备列表': 'Device List',
        '注册设备': 'Register Device',
        '设备别名': 'Device alias',
        '创建': 'Create',
        '保存': 'Save',
        '取消': 'Cancel',
        '改名': 'Rename',
        '启用': 'Enable',
        '禁用': 'Disable',
        '轮换': 'Rotate',
        '删除': 'Delete',
        '复制': 'Copy',
        '已复制': 'Copied',
        '新设备密钥': 'New device secret',
        '目标设备': 'Target device',
        '在线设备': 'Online devices',
        'GPU 总数': 'Total GPUs',
        '忙碌 GPU': 'Busy GPUs',
        '高温 GPU': 'Hot GPUs',
        '总显存用量': 'Total memory usage',
        '总功耗': 'Total power',
        'GPU 进程': 'GPU Processes',
        '24 小时统计': '24-hour Stats',
        '维护与发布': 'Maintenance & Release',
        '访问与安全': 'Access & Security',
        '作者': 'Author',
        '版本': 'Version',
        '提交': 'Commit',
        '构建时间': 'Build time',
        '仓库地址': 'Repository',
        '新增': 'Added',
        '变更': 'Changed',
        '安全': 'Security',
        '修复': 'Fixed'
      }
    };
    function fallbackTranslateText(value) {
      if (fallbackI18n.language !== 'en-US') return value;
      const trimmed = String(value || '').trim();
      if (!trimmed) return value;
      const exact = fallbackI18n.exact[trimmed];
      if (exact) return String(value).replace(trimmed, exact);
      return String(value)
        .replace(/(\d+) 台设备，(\d+) 块 GPU，按最新上报状态汇总。/, '$1 devices, $2 GPUs, summarized from latest reports.')
        .replace(/服务端时间 (.+)/, 'Server time $1')
        .replace(/数据库大小 (.+) · 已存储 (.+) 天 · (.+) 空闲/, 'Database size $1 · stored $2 days · $3 free')
        .replace(/数据库大小 (.+)/, 'Database size $1')
        .replace(/空闲 (.+)/, '$1 free')
        .replace(/当前证书到期：(.+)/, 'Current certificate expires: $1')
        .replace(/已创建设备 (.+)/, 'Created device $1')
        .replace(/设备名称已更新为 (.+)/, 'Device name updated to $1');
    }
    function applyFallbackI18n() {
      document.documentElement.lang = fallbackI18n.language === 'en-US' ? 'en' : 'zh-CN';
      const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
      const nodes = [];
      while (walker.nextNode()) nodes.push(walker.currentNode);
      nodes.forEach((node) => {
        const current = node.nodeValue || '';
        const translated = fallbackTranslateText(node.__sourceText || current);
        if (!node.__sourceText || (current !== node.__sourceText && current !== translated)) node.__sourceText = current;
        const next = fallbackTranslateText(node.__sourceText);
        if (node.nodeValue !== next) node.nodeValue = next;
      });
      document.querySelectorAll('[title],[placeholder],[aria-label]').forEach((el) => {
        ['title', 'placeholder', 'aria-label'].forEach((attr) => {
          const value = el.getAttribute(attr);
          if (!value) return;
          const key = '__source_' + attr;
          const translated = fallbackTranslateText(el[key] || value);
          if (!el[key] || (value !== el[key] && value !== translated)) el[key] = value;
          el.setAttribute(attr, fallbackTranslateText(el[key]));
        });
      });
    }
    const login = document.getElementById('login');
    const app = document.getElementById('app');
    const state = {
      view: 'overview',
      data: null,
      stats: [],
      history: new Map(),
      message: '',
      secret: null,
      pendingConfirm: null,
      confirmBusy: false,
      editingDevice: null,
      updateCheckTimer: null,
      updateDetail: '',
      theme: initialTheme()
    };
    const updateStatusCacheKey = 'gpufleet-update-status-cache';
    const updateStatusCacheTTL = 60 * 60 * 1000;
    const titles = {
      overview: 'GPU 资源总览',
      devices: '设备管理',
      gpus: 'GPU 监控',
      settings: '服务设置'
    };
    const deviceBorderPalette = ['#146c78', '#6750a4', '#b26a00', '#198754', '#c54040', '#2f6fbd', '#8a5a00', '#00806a'];
    setTheme(state.theme);

    function initialTheme() {
      const stored = localStorage.getItem('gpufleet-theme');
      if (stored === 'light' || stored === 'dark') return stored;
      return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }
    function setTheme(theme) {
      state.theme = theme;
      document.documentElement.dataset.theme = theme;
      document.documentElement.style.colorScheme = theme;
      localStorage.setItem('gpufleet-theme', theme);
    }
    function toggleTheme() {
      setTheme(state.theme === 'dark' ? 'light' : 'dark');
    }
    document.getElementById('themeToggle').addEventListener('click', toggleTheme);
    document.getElementById('loginTheme').addEventListener('click', toggleTheme);

    const fmtBytes = (n) => {
      if (typeof n !== 'number' || !Number.isFinite(n)) return '-';
      const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
      let i = 0;
      let v = n;
      while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
      return v.toFixed(i ? 1 : 0) + ' ' + units[i];
    };
    const fmtMemoryG = (used, total) => {
      const usedValid = typeof used === 'number' && Number.isFinite(used);
      const totalValid = typeof total === 'number' && Number.isFinite(total) && total > 0;
      const toG = (value) => (value / 1024 / 1024 / 1024).toFixed(1);
      if (usedValid && totalValid) return toG(used) + '/' + toG(total) + ' G';
      if (usedValid) return toG(used) + ' G';
      if (totalValid) return '0.0/' + toG(total) + ' G';
      return '-';
    };
    const pct = (n) => typeof n === 'number' && Number.isFinite(n) ? Math.round(n) + '%' : '-';
    const watts = (n) => typeof n === 'number' && Number.isFinite(n) ? n.toFixed(1) + ' W' : '-';
    const temp = (n) => typeof n === 'number' && Number.isFinite(n) ? Math.round(n) + '°C' : '-';
    const mhz = (n) => typeof n === 'number' && Number.isFinite(n) ? Math.round(n) + ' MHz' : '-';
    const fmtStoredDays = (days, fallbackHours) => {
      const fallbackDays = typeof fallbackHours === 'number' && Number.isFinite(fallbackHours) && fallbackHours > 0 ? Math.ceil(fallbackHours / 24) : 0;
      const value = typeof days === 'number' && Number.isFinite(days) ? days : fallbackDays;
      return '已存储 ' + Math.max(0, value) + ' 天';
    };
    const esc = (value) => String(value == null ? '' : value).replace(/[&<>"']/g, (c) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));

    async function api(url, options) {
      const res = await fetch(url, {
        headers: {'Content-Type': 'application/json', ...((options && options.headers) || {})},
        credentials: 'same-origin',
        ...(options || {})
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        if (typeof body.retry_after_seconds === 'number' && body.retry_after_seconds > 0) {
          throw new Error('请求过于频繁，请等待 ' + fmtRetryAfter(body.retry_after_seconds) + '后再试');
        }
        throw new Error(body.error || res.statusText);
      }
      return res.json();
    }
    function fmtRetryAfter(seconds) {
      const rounded = Math.max(1, Math.ceil(seconds));
      if (rounded >= 3600) return Math.ceil(rounded / 3600) + ' 小时';
      if (rounded >= 60) return Math.ceil(rounded / 60) + ' 分钟';
      return rounded + ' 秒';
    }

    document.getElementById('loginForm').addEventListener('submit', async (event) => {
      event.preventDefault();
      const form = new FormData(event.currentTarget);
      try {
        await api('/api/v1/auth/login', {
          method: 'POST',
          body: JSON.stringify({password: form.get('password')})
        });
        document.getElementById('loginError').textContent = '';
        await refresh();
      } catch (err) {
        document.getElementById('loginError').textContent = err.message;
      }
    });
    document.getElementById('refresh').addEventListener('click', refresh);
    document.getElementById('logout').addEventListener('click', async () => {
      await api('/api/v1/auth/logout', {method: 'POST'}).catch(() => undefined);
      app.classList.add('hidden');
      login.classList.remove('hidden');
    });
    document.querySelectorAll('[data-view]').forEach((button) => {
      button.addEventListener('click', () => setView(button.dataset.view));
    });
    document.addEventListener('click', async (event) => {
      const button = event.target.closest('[data-device-action]');
      if (!button) return;
      openDeviceConfirm(button.dataset.deviceId, button.dataset.deviceAction);
    });
    document.addEventListener('click', (event) => {
      const button = event.target.closest('[data-device-edit]');
      if (!button) return;
      const device = (state.data && state.data.devices || []).find((item) => item.id === button.dataset.deviceId);
      if (!device) return;
      state.editingDevice = {id: device.id, alias: device.alias || device.id};
      render();
    });
    document.addEventListener('click', (event) => {
      const button = event.target.closest('[data-rename-cancel]');
      if (!button) return;
      state.editingDevice = null;
      render();
    });
    document.addEventListener('submit', async (event) => {
      const form = event.target.closest('[data-device-rename-form]');
      if (!form) return;
      event.preventDefault();
      const deviceID = form.dataset.deviceId;
      const alias = new FormData(form).get('alias');
      try {
        const result = await api('/api/v1/admin/devices/' + encodeURIComponent(deviceID), {method: 'PATCH', body: JSON.stringify({alias: String(alias || '').trim()})});
        state.message = '设备名称已更新为 ' + (result.device.alias || result.device.id);
        state.editingDevice = null;
        await refresh();
      } catch (err) {
        state.message = err.message;
        render();
      }
    });
    document.addEventListener('click', async (event) => {
      const action = event.target.closest('[data-confirm-action]');
      if (!action) return;
      if (action.dataset.confirmAction === 'cancel') {
        closeDeviceConfirm();
        return;
      }
      if (action.dataset.confirmAction === 'confirm') await runDeviceConfirm();
    });
    document.addEventListener('mousedown', (event) => {
      if (event.target && event.target.classList && event.target.classList.contains('modal-backdrop')) closeDeviceConfirm();
    });
    document.addEventListener('keydown', (event) => {
      if (event.key === 'Escape') closeDeviceConfirm();
    });
    document.addEventListener('pointermove', (event) => {
      const wrap = event.target.closest('.sparkline-wrap');
      if (wrap) updateSparkHover(wrap, event.clientX);
    });
    document.addEventListener('pointerout', (event) => {
      const wrap = event.target.closest('.sparkline-wrap');
      if (wrap && !wrap.contains(event.relatedTarget)) clearSparkHover(wrap);
    });

    function setView(view) {
      state.view = view;
      document.querySelectorAll('[data-view]').forEach((button) => {
        button.classList.toggle('active', button.dataset.view === view);
      });
      document.querySelectorAll('[data-page]').forEach((section) => {
        section.classList.toggle('hidden', section.dataset.page !== view);
      });
      document.getElementById('viewTitle').textContent = titles[view];
      render();
    }

    async function refresh() {
      try {
        const data = await api('/api/v1/overview');
        const stats = await api('/api/v1/stats/gpu-utilization?hours=24').catch(() => ({stats: []}));
        state.data = data;
        state.stats = stats.stats || [];
        updateHistory(data.latest_gpus || []);
        login.classList.add('hidden');
        app.classList.remove('hidden');
        render();
      } catch (err) {
        if (err.message.includes('login')) {
          app.classList.add('hidden');
          login.classList.remove('hidden');
        } else {
          document.getElementById('banner').innerHTML = '<div class="banner danger">' + esc(err.message) + '</div>';
        }
      }
    }

    function updateHistory(items) {
      items.forEach((item) => {
        const gpu = item.gpu || {};
        const key = item.device_id + '/' + gpu.gpu_id;
        const mem = gpu.memory_total_bytes ? gpu.memory_used_bytes / gpu.memory_total_bytes * 100 : undefined;
        pushHistory(key + ':util', gpu.utilization_gpu_percent, item.timestamp);
        pushHistory(key + ':mem', mem, item.timestamp);
        pushHistory(key + ':temp', gpu.temperature_celsius, item.timestamp);
        pushHistory(key + ':power', gpu.power_draw_watts, item.timestamp);
      });
    }
    function pushHistory(key, value, timestamp) {
      if (typeof value !== 'number' || !Number.isFinite(value)) return;
      const values = state.history.get(key) || [];
      values.push({value, timestamp});
      while (values.length > 24) values.shift();
      state.history.set(key, values);
    }
    function history(key, fallback) {
      const values = state.history.get(key) || [];
      if (values.length >= 2) return values;
      if (typeof fallback === 'number' && Number.isFinite(fallback)) return [{value: fallback}, {value: fallback}];
      return [{value: 0}, {value: 0}];
    }

    function render() {
      const data = state.data;
      if (!data) return;
      document.getElementById('serverTime').textContent = '服务端时间 ' + new Date(data.server_time).toLocaleString();
      document.getElementById('banner').innerHTML = data.disk && data.disk.status === 'critical'
        ? '<div class="banner danger">磁盘空间低于保护阈值，服务端已拒绝新指标写入。</div>'
        : '';
      if (state.view === 'overview') renderOverview(data);
      if (state.view === 'devices') renderDevicesPage(data);
      if (state.view === 'gpus') renderGPUPage(data);
      if (state.view === 'settings') renderSettings(data);
      if (data.service && data.service.language) {
        fallbackI18n.language = data.service.language;
        localStorage.setItem('gpufleet-language', fallbackI18n.language);
      }
      applyFallbackI18n();
    }

    function renderOverview(data) {
      const gpus = data.latest_gpus || [];
      const devices = data.devices || [];
      const hot = gpus.filter((item) => ((item.gpu || {}).temperature_celsius || 0) >= 80).length;
      const busy = gpus.filter((item) => ((item.gpu || {}).utilization_gpu_percent || 0) >= 80).length;
      document.getElementById('overviewView').innerHTML =
        '<section class="fleet-command">' +
          '<div class="fleet-command-copy"><span class="fleet-eyebrow">Fleet Live</span><h2>多机 GPU 运行态</h2><p>' +
          (devices.length ? devices.length + ' 台设备，' + gpus.length + ' 块 GPU，按最新上报状态汇总。' : '等待客户端上报 GPU 运行信息。') +
          '</p></div>' +
          '<div class="fleet-kpis">' +
            fleetKPI('在线设备', (data.online_device_count || 0) + '/' + (data.device_count || 0), (data.online_device_count || 0) === (data.device_count || 0) ? 'good' : 'warn') +
            fleetKPI('GPU 总数', String(data.gpu_count || 0), '') +
            fleetKPI('忙碌 GPU', String(busy), busy ? 'accent' : 'good') +
            fleetKPI('高温 GPU', String(hot), hot ? 'bad' : 'good') +
            fleetKPI('总显存用量', fmtMemoryG(data.memory_used_bytes, data.memory_total_bytes), '') +
            fleetKPI('总功耗', watts(data.power_draw_watts || 0), data.power_draw_watts ? 'accent' : 'good') +
          '</div>' +
        '</section>' +
        '<section class="overview-layout">' +
          '<section class="fleet-board panel" data-testid="fleet-board"><div class="panel-head"><div><h2>GPU Fleet</h2><p>卡片化查看多设备 GPU 运行状态</p></div><span>' + gpus.length + ' GPUs</span></div><div class="fleet-card-grid">' + renderFleetCards(gpus, devices) + '</div></section>' +
          '<div class="stack">' + renderDeviceList(data) + renderProcessList(data.latest_processes || [], data.devices || []) + '</div>' +
        '</section>' +
        '<section class="overview-secondary">' + renderStatsTable(state.stats, state.overview && state.overview.devices || []) + '</section>';
    }

    function fleetKPI(label, value, tone) {
      return '<div class="fleet-kpi ' + tone + '"><span>' + esc(label) + '</span><strong>' + esc(value) + '</strong></div>';
    }

    function renderFleetCards(items, devices) {
      const deviceMap = new Map((devices || []).map((d) => [d.id, d]));
      if (!items.length) return '<p class="empty">暂无 GPU 上报</p>';
      return items.map((item) => {
        const gpu = item.gpu || {};
        const device = deviceMap.get(item.device_id);
        const health = gpuHealth(item, device);
        const key = item.device_id + '/' + gpu.gpu_id;
        const mem = gpu.memory_total_bytes ? gpu.memory_used_bytes / gpu.memory_total_bytes * 100 : undefined;
        const color = deviceBorderColor(item.device_id);
        return '<article class="fleet-gpu-card ' + health.tone + '" data-testid="fleet-gpu-card" data-device-id="' + esc(item.device_id) + '" data-device-color="' + color + '" style="--device-color:' + color + '">' +
          (health.tone === 'offline' ? '<div class="offline-mask">离线</div>' : '') +
          '<div class="fleet-card-top"><div class="fleet-device-cell"><span class="status-dot ' + health.tone + '"></span><div><strong>' + esc(deviceName(device, item.device_id)) + '</strong><p>' + esc(shortGPUName(gpu.name || gpu.gpu_id || '-') + ' · ' + (gpu.gpu_id || '-') + ' · ' + timeAgo(item.timestamp)) + '</p></div></div><span class="pill ' + health.tone + '">' + health.label + '</span></div>' +
          '<div class="gpu-card-meta"><span>' + esc(pcieLabel(gpu)) + '</span><span>' + esc(gpu.pstate || '-') + '</span><span>' + esc(gpu.compute_capability ? 'Compute ' + gpu.compute_capability : gpu.driver_model || '-') + '</span></div>' +
          '<div class="gpu-trend-grid">' +
            trendTile('GPU 利用率', pct(gpu.utilization_gpu_percent), gpu.sm_clock_mhz ? mhz(gpu.sm_clock_mhz) : '刷新历史', history(key + ':util', gpu.utilization_gpu_percent), 100, metricTone(gpu.utilization_gpu_percent, 70, 92), '%') +
            trendTile('显存', pct(mem) + ' · ' + fmtBytes(gpu.memory_used_bytes), '总量 ' + fmtBytes(gpu.memory_total_bytes), history(key + ':mem', mem), 100, metricTone(mem, 75, 92), '%') +
            trendTile('温度', temp(gpu.temperature_celsius), tempToneText(gpu.temperature_celsius), history(key + ':temp', gpu.temperature_celsius), 100, metricTone(gpu.temperature_celsius, 80, 88), '°C') +
            trendTile('功耗', watts(gpu.power_draw_watts), gpu.power_limit_watts ? '上限 ' + watts(gpu.power_limit_watts) : gpu.pstate || '-', history(key + ':power', gpu.power_draw_watts), gpu.power_limit_watts || maxOf(history(key + ':power', gpu.power_draw_watts), 200), metricTone(gpu.power_limit_watts && gpu.power_draw_watts ? gpu.power_draw_watts / gpu.power_limit_watts * 100 : undefined, 78, 95), ' W') +
          '</div></article>';
      }).join('');
    }

    function trendTile(label, value, caption, values, max, tone, unit) {
      return '<div class="trend-tile ' + tone + '" data-testid="gpu-trend-tile"><div class="trend-head"><div><span>' + esc(label) + '</span><strong>' + esc(value) + '</strong></div><p>' + esc(caption) + '</p></div>' + spark(label, values, max, unit) + '</div>';
    }
    function spark(label, values, max, unit) {
      const width = 180;
      const height = 74;
      const pad = 8;
      const capped = Math.max(max || 100, 1);
      const list = values && values.length ? values : [0, 0];
      const dataValues = list.map((sample) => Number.isFinite(sample.value) ? sample.value : 0).join(',');
      const dataTimes = list.map((sample) => sample.timestamp || '').join('|');
      const points = list.map((sample, i) => {
        const v = sample.value;
        const x = pad + (i / Math.max(1, list.length - 1)) * (width - pad * 2);
        const y = height - pad - (Math.max(0, Math.min(capped, v || 0)) / capped) * (height - pad * 2);
        return x.toFixed(1) + ',' + y.toFixed(1);
      }).join(' ');
      const area = pad + ',' + (height - pad) + ' ' + points + ' ' + (width - pad) + ',' + (height - pad);
      return '<div class="sparkline-wrap" data-values="' + dataValues + '" data-times="' + esc(dataTimes) + '" data-max="' + capped + '" data-label="' + esc(label) + '" data-unit="' + esc(unit || '') + '">' +
        '<svg class="sparkline" viewBox="0 0 ' + width + ' ' + height + '" role="img" aria-label="' + esc(label) + ' 历史趋势图" preserveAspectRatio="none"><polyline class="spark-grid" points="' + pad + ',' + (height - pad) + ' ' + (width - pad) + ',' + (height - pad) + '"></polyline><polygon class="spark-area" points="' + area + '"></polygon><polyline class="spark-line" points="' + points + '"></polyline><line class="spark-cursor" x1="' + pad + '" x2="' + pad + '" y1="' + pad + '" y2="' + (height - pad) + '" style="display:none"></line><circle class="spark-point" cx="' + pad + '" cy="' + (height - pad) + '" r="3.2" style="display:none"></circle></svg>' +
        '<div class="spark-tooltip" data-testid="spark-tooltip"><span>' + esc(label) + '</span><strong>-</strong><small>-</small></div></div>';
    }

    function updateSparkHover(wrap, clientX) {
      const values = (wrap.dataset.values || '').split(',').map(Number).filter((value) => Number.isFinite(value));
      if (!values.length) return;
      const times = (wrap.dataset.times || '').split('|');
      const width = 180;
      const height = 74;
      const pad = 8;
      const max = Math.max(Number(wrap.dataset.max) || 100, 1);
      const rect = wrap.getBoundingClientRect();
      const ratio = rect.width > 0 ? (clientX - rect.left) / rect.width : 0;
      const index = Math.max(0, Math.min(values.length - 1, Math.round(ratio * (values.length - 1))));
      const x = pad + (index / Math.max(1, values.length - 1)) * (width - pad * 2);
      const y = height - pad - (Math.max(0, Math.min(max, values[index])) / max) * (height - pad * 2);
      const cursor = wrap.querySelector('.spark-cursor');
      const point = wrap.querySelector('.spark-point');
      const tooltip = wrap.querySelector('.spark-tooltip');
      cursor.setAttribute('x1', x.toFixed(1));
      cursor.setAttribute('x2', x.toFixed(1));
      cursor.style.display = '';
      point.setAttribute('cx', x.toFixed(1));
      point.setAttribute('cy', y.toFixed(1));
      point.style.display = '';
      tooltip.style.left = (x / width * 100).toFixed(1) + '%';
      tooltip.querySelector('span').textContent = wrap.dataset.label || '';
      tooltip.querySelector('strong').textContent = formatSparkValue(values[index], wrap.dataset.unit || '');
      tooltip.querySelector('small').textContent = times[index] ? new Date(times[index]).toLocaleString() : '-';
      tooltip.classList.add('show');
    }

    function clearSparkHover(wrap) {
      const cursor = wrap.querySelector('.spark-cursor');
      const point = wrap.querySelector('.spark-point');
      const tooltip = wrap.querySelector('.spark-tooltip');
      if (cursor) cursor.style.display = 'none';
      if (point) point.style.display = 'none';
      if (tooltip) tooltip.classList.remove('show');
    }

    function formatSparkValue(value, unit) {
      if (!Number.isFinite(value)) return '-';
      if (unit === ' W') return value.toFixed(1) + unit;
      if (unit === '%' || unit === '°C') return Math.round(value) + unit;
      return (Math.abs(value) >= 100 ? Math.round(value) : value.toFixed(1)) + unit;
    }

    function openDeviceConfirm(deviceID, action) {
      const device = (state.data && state.data.devices || []).find((item) => item.id === deviceID);
      if (!device) return;
      state.pendingConfirm = {device, action};
      renderDeviceConfirm();
    }

    function closeDeviceConfirm() {
      state.pendingConfirm = null;
      state.confirmBusy = false;
      renderDeviceConfirm();
    }

    async function runDeviceConfirm() {
      if (!state.pendingConfirm || state.confirmBusy) return;
      const pending = state.pendingConfirm;
      const id = pending.device.id;
      const action = pending.action;
      state.confirmBusy = true;
      renderDeviceConfirm();
      try {
        let result = null;
        if (action === 'delete') {
          result = await api('/api/v1/admin/devices/' + encodeURIComponent(id), {method: 'DELETE'});
          state.secret = null;
          state.message = '设备已删除';
        } else {
          const endpoint = action === 'rotate' ? 'rotate-secret' : action;
          result = await api('/api/v1/admin/devices/' + encodeURIComponent(id) + '/' + endpoint, {method: 'POST', body: '{}'});
          state.message = action === 'disable' ? '设备已禁用' : action === 'enable' ? '设备已启用' : '设备密钥已轮换';
          if (result && result.secret) state.secret = {deviceId: id, value: result.secret, title: '已轮换密钥'};
        }
        closeDeviceConfirm();
        await refresh();
      } catch (err) {
        state.message = err.message;
        closeDeviceConfirm();
        render();
      }
    }

    function renderDeviceConfirm() {
      const root = document.getElementById('modalRoot');
      if (!root) return;
      const pending = state.pendingConfirm;
      if (!pending) {
        root.innerHTML = '';
        return;
      }
      const copy = deviceConfirmCopy(pending.action, pending.device);
      root.innerHTML =
        '<div class="modal-backdrop">' +
          '<section class="confirm-dialog ' + esc(copy.tone) + '" role="dialog" aria-modal="true" data-testid="device-confirm-dialog">' +
            '<div class="confirm-icon">!</div>' +
            '<div class="confirm-copy"><span>' + esc(pending.device.id) + '</span><h2>' + esc(copy.title) + '</h2><p>' + esc(copy.body) + '</p></div>' +
            '<div class="confirm-target"><span>目标设备</span><strong>' + esc(pending.device.alias || pending.device.id) + '</strong></div>' +
            '<div class="confirm-actions"><button class="secondary" type="button" data-confirm-action="cancel" ' + (state.confirmBusy ? 'disabled' : '') + '>取消</button><button class="primary narrow ' + (copy.tone === 'danger' ? 'danger-primary' : '') + '" type="button" data-confirm-action="confirm" ' + (state.confirmBusy ? 'disabled' : '') + '>' + esc(state.confirmBusy ? '处理中' : copy.confirmLabel) + '</button></div>' +
          '</section>' +
        '</div>';
    }

    function deviceConfirmCopy(action, device) {
      const name = device.alias || device.id;
      if (action === 'enable') return {
        title: '启用设备',
        body: '允许 ' + name + ' 使用现有密钥继续上报 GPU 指标。',
        confirmLabel: '确认启用',
        tone: 'normal'
      };
      if (action === 'disable') return {
        title: '禁用设备',
        body: '禁用后 ' + name + ' 的上报请求会被服务端拒绝，客户端本机配置不会被修改。',
        confirmLabel: '确认禁用',
        tone: 'warning'
      };
      if (action === 'rotate') return {
        title: '轮换密钥',
        body: '旧密钥会立即失效，需要在 ' + name + ' 所在机器手动更新新密钥后才能继续上报。',
        confirmLabel: '确认轮换',
        tone: 'warning'
      };
      return {
        title: '删除设备',
        body: '删除后 ' + name + ' 将从设备列表和最新 GPU 快照中移除，原 Agent 密钥会失效。',
        confirmLabel: '确认删除',
        tone: 'danger'
      };
    }

    function renderGPUPage(data) {
      const gpus = data.latest_gpus || [];
      document.getElementById('gpusView').innerHTML =
        '<section class="metric-grid">' +
          metric('在线设备', (data.online_device_count || 0) + ' / ' + (data.device_count || 0)) +
          metric('GPU 数量', String(data.gpu_count || 0)) +
          metric('平均利用率', pct(data.average_utilization || 0)) +
          metric('总显存用量', fmtMemoryG(data.memory_used_bytes, data.memory_total_bytes)) +
          metric('总功耗', watts(data.power_draw_watts || 0)) +
        '</section>' +
        '<section class="main-grid"><div class="panel"><div class="panel-head"><h2>GPU 详细状态</h2><span>' + gpus.length + '</span></div><div class="gpu-grid">' + renderGPUCards(gpus) + '</div></div><div class="stack">' + renderDeviceList(data) + renderProcessList(data.latest_processes || [], data.devices || []) + '</div></section>' +
        renderStatsTable(state.stats, data.devices || []);
    }
    function metric(label, value) {
      return '<article class="metric"><p>' + esc(label) + '</p><strong>' + esc(value) + '</strong></article>';
    }
    function renderGPUCards(items) {
      if (!items.length) return '<p class="empty">暂无 GPU 上报</p>';
      return items.map((item) => {
        const gpu = item.gpu || {};
        const key = item.device_id + '/' + gpu.gpu_id;
        const mem = gpu.memory_total_bytes ? gpu.memory_used_bytes / gpu.memory_total_bytes * 100 : undefined;
        const color = deviceBorderColor(item.device_id);
        const rows = [
          ['显存空闲', fmtBytes(gpu.memory_free_bytes)],
          ['显存保留', fmtBytes(gpu.memory_reserved_bytes)],
          ['显存利用', pct(gpu.utilization_memory_percent)],
          ['温度上限', temp(gpu.temperature_limit_celsius)],
          ['显存温度', temp(gpu.temperature_memory_celsius)],
          ['功耗上限', watts(gpu.power_limit_watts || gpu.power_enforced_limit_watts)],
          ['风扇', pct(gpu.fan_speed_percent)],
          ['图形时钟', mhz(gpu.graphics_clock_mhz)],
          ['显存时钟', mhz(gpu.memory_clock_mhz)],
          ['SM 时钟', mhz(gpu.sm_clock_mhz)],
          ['视频时钟', mhz(gpu.video_clock_mhz)],
          ['P-State', gpu.pstate || '-'],
          ['PCIe 当前', pcieLabel(gpu)],
          ['Compute', gpu.compute_capability || gpu.compute_mode || '-'],
          ['显示', [gpu.display_active, gpu.display_attached].filter(Boolean).join(' / ') || '-'],
          ['驱动模型', gpu.driver_model || '-'],
          ['VBIOS', gpu.vbios_version || '-'],
          ['ECC', gpu.ecc_mode_current || '-'],
          ['MIG', gpu.mig_mode_current || '-']
        ].filter((row) => row[1] !== '-');
        return '<article class="gpu-card" data-device-id="' + esc(item.device_id) + '" data-device-color="' + color + '" style="--device-color:' + color + '"><div class="card-title"><div><h3>' + esc(gpu.name || gpu.gpu_id || '-') + '</h3><p>' + esc(item.device_id + ' · ' + (gpu.gpu_id || '-')) + '</p></div><span class="pill">' + pct(gpu.utilization_gpu_percent) + '</span></div>' +
          '<div class="gpu-detail-trend-grid">' +
            trendTile('GPU 利用率', pct(gpu.utilization_gpu_percent), gpu.sm_clock_mhz ? mhz(gpu.sm_clock_mhz) : '刷新历史', history(key + ':util', gpu.utilization_gpu_percent), 100, metricTone(gpu.utilization_gpu_percent, 70, 92), '%') +
            trendTile('显存', pct(mem) + ' · ' + fmtBytes(gpu.memory_used_bytes), '总量 ' + fmtBytes(gpu.memory_total_bytes), history(key + ':mem', mem), 100, metricTone(mem, 75, 92), '%') +
            trendTile('温度', temp(gpu.temperature_celsius), tempToneText(gpu.temperature_celsius), history(key + ':temp', gpu.temperature_celsius), 100, metricTone(gpu.temperature_celsius, 80, 88), '°C') +
            trendTile('功耗', watts(gpu.power_draw_watts), gpu.power_limit_watts ? '上限 ' + watts(gpu.power_limit_watts) : gpu.pstate || '-', history(key + ':power', gpu.power_draw_watts), gpu.power_limit_watts || maxOf(history(key + ':power', gpu.power_draw_watts), 200), metricTone(gpu.power_limit_watts && gpu.power_draw_watts ? gpu.power_draw_watts / gpu.power_limit_watts * 100 : undefined, 78, 95), ' W') +
          '</div><div class="gpu-detail-grid">' + rows.map((row) => '<div><span>' + esc(row[0]) + '</span><strong>' + esc(row[1]) + '</strong></div>').join('') + '</div></article>';
      }).join('');
    }

    function renderDevicesPage(data) {
      const devices = data.devices || [];
      document.getElementById('devicesView').innerHTML =
        '<div class="device-admin">' +
          '<section class="panel"><div class="panel-head"><h2>注册设备</h2><span>' + devices.length + '</span></div><form class="device-form" id="createDeviceForm"><label>设备别名<input name="alias" placeholder="worker-a100-01"></label><button class="primary narrow" type="submit">创建</button></form>' + renderDeviceMessage() + '</section>' +
          '<section class="panel"><div class="panel-head"><h2>设备列表</h2><span>' + devices.length + '</span></div>' + devices.map(renderDeviceRow).join('') + (devices.length ? '' : '<p class="empty">暂无设备</p>') + '</section>' +
        '</div>';
      const form = document.getElementById('createDeviceForm');
      form.addEventListener('submit', async (event) => {
        event.preventDefault();
        const alias = new FormData(form).get('alias');
        try {
          const result = await api('/api/v1/admin/devices', {method: 'POST', body: JSON.stringify({alias: String(alias || '').trim()})});
          state.message = '已创建设备 ' + (result.device.alias || result.device.id);
          state.secret = {deviceId: result.device.id, value: result.secret, title: '新设备密钥'};
          await refresh();
        } catch (err) {
          state.message = err.message;
          render();
        }
      });
    }
    function renderDeviceMessage() {
      const message = state.message ? '<p class="' + (state.message.includes('failed') || state.message.includes('error') ? 'error' : 'notice') + '">' + esc(state.message) + '</p>' : '';
      const secret = state.secret ? '<div class="secret-box"><div><strong>' + esc(state.secret.title) + '</strong><p>' + esc(state.secret.deviceId) + '</p></div><code>' + esc(state.secret.value) + '</code><button class="secondary" type="button" onclick="navigator.clipboard && navigator.clipboard.writeText(' + "'" + escJS(state.secret.value) + "'" + ')">复制</button></div>' : '';
      return message + secret;
    }
    function renderDeviceRow(device) {
      const status = device.enabled ? (device.status || 'offline') : 'disabled';
      const action = device.enabled ? 'disable' : 'enable';
      const editing = state.editingDevice && state.editingDevice.id === device.id;
      const nameCell = editing
        ? '<form class="device-rename-form" data-device-rename-form data-device-id="' + esc(device.id) + '"><input name="alias" value="' + esc(state.editingDevice.alias) + '" aria-label="设备名称"><button class="secondary" type="submit">保存</button><button class="secondary" type="button" data-rename-cancel>取消</button></form>'
        : '<strong>' + esc(device.alias || device.id) + '</strong><p>' + esc(device.id + ' · ' + (device.hostname || '-') + ' · ' + (device.agent_version || '-')) + '</p>';
      return '<div class="device-row"><div class="device-name-cell">' + nameCell + '</div><span class="pill ' + status + '">' + status + '</span><div class="row-actions"><button class="secondary" data-device-edit data-device-id="' + esc(device.id) + '"' + (state.editingDevice ? ' disabled' : '') + '>改名</button><button class="secondary" data-device-action="' + action + '" data-device-id="' + esc(device.id) + '"' + (state.editingDevice ? ' disabled' : '') + '>' + (device.enabled ? '禁用' : '启用') + '</button><button class="secondary" data-device-action="rotate" data-device-id="' + esc(device.id) + '"' + (state.editingDevice ? ' disabled' : '') + '>轮换</button><button class="secondary danger-action" data-device-action="delete" data-device-id="' + esc(device.id) + '"' + (state.editingDevice ? ' disabled' : '') + '>删除</button></div></div>';
    }
    function escJS(value) {
      return String(value || '').replace(/\\/g, '\\\\').replace(/'/g, "\\'");
    }

    function renderDeviceList(data) {
      const devices = data.devices || [];
      return '<section class="panel"><div class="panel-head"><h2>设备</h2><span>' + devices.length + '</span></div>' +
        devices.map((device) => '<div class="list-row"><div><strong>' + esc(device.alias || device.id) + '</strong><p>' + esc([device.hostname, device.os, device.agent_version].filter(Boolean).join(' · ') || device.id) + '</p></div><span class="pill ' + (device.enabled ? (device.status || 'offline') : 'disabled') + '">' + esc(device.enabled ? (device.status || 'offline') : 'disabled') + '</span></div>').join('') +
        (devices.length ? '' : '<p class="empty">暂无设备</p>') + '</section>';
    }
    function renderProcessList(items, devices) {
      const deviceMap = new Map((devices || []).map((device) => [device.id, device]));
      return '<section class="panel"><div class="panel-head"><h2>GPU 进程</h2><span>' + items.length + '</span></div>' +
        items.slice(0, 32).map((item) => '<div class="list-row"><div><strong>' + esc((item.process || {}).process_name || 'unknown') + '</strong><p>' + esc(deviceName(deviceMap.get(item.device_id), item.device_id || '-')) + ' · PID ' + esc((item.process || {}).pid || '-') + ' · ' + esc((item.process || {}).gpu_id || '-') + '</p></div><span class="pill">' + fmtBytes((item.process || {}).used_memory_bytes) + '</span></div>').join('') +
        (items.length ? '' : '<p class="empty">暂无 GPU 进程快照</p>') + '</section>';
    }
    function renderStatsTable(rows, devices) {
      const deviceMap = new Map((devices || []).map((device) => [device.id, device]));
      return '<section class="panel"><div class="panel-head"><h2>24 小时统计</h2><span>' + rows.length + '</span></div><div class="stats-table">' +
        rows.map((row) => '<div class="table-row"><div><strong>' + esc(row.gpu_name || row.gpu_id) + '</strong><p>' + esc(deviceName(deviceMap.get(row.device_id), row.device_id || '-') + ' · ' + row.gpu_id + ' · ' + row.sample_count + ' samples') + '</p></div><span>' + pct(row.average_utilization_percent) + '</span><span>' + pct(row.idle_sample_percent) + ' idle</span><span>' + temp(row.peak_temperature_celsius) + '</span><span>' + watts(row.peak_power_draw_watts) + '</span></div>').join('') +
        (rows.length ? '' : '<p class="empty">暂无统计数据</p>') + '</div></section>';
    }

    function renderSettings(data) {
      const disk = data.disk || {};
      const service = data.service || {};
      const min = data.min_free_space_bytes || disk.min_free_bytes || 0;
      const certCaption = service.https_enabled ? (service.current_scheme === 'https' ? 'HTTPS 已启用' : 'HTTPS 下次启动生效') : 'HTTP 模式';
      document.getElementById('settingsView').innerHTML =
        '<div class="settings-page" data-testid="settings-page">' +
          '<section class="settings-status panel"><div class="panel-head"><div><h2>服务状态</h2><p>' + esc((service.current_addr || '-') + ' · ' + String(service.current_scheme || 'http').toUpperCase()) + '</p></div>' + (service.restart_required ? '<span class="pill warn">需要重启</span>' : '') + '</div>' +
          '<div class="settings-kpi-grid">' +
            settingStat('当前协议', String(service.current_scheme || 'http').toUpperCase(), service.https_enabled ? '证书已配置' : '未启用证书') +
            settingStat('访问端口', String(service.configured_port || location.port || 8080), service.current_addr || '-') +
            settingStat('证书到期', service.cert_not_after ? new Date(service.cert_not_after).toLocaleString() : '未配置', certCaption) +
            settingStat('磁盘预留', fmtBytes(min), '空闲 ' + fmtBytes(disk.free_bytes)) +
          '</div></section>' +
          '<section class="settings-workbench">' +
            '<div class="settings-column">' +
              settingsSectionHead('访问与安全', '凭据、端口和 HTTPS 证书') +
              operationPanel('密码更改', '仅使用密码作为 Web 凭据', 'PW', '<button class="secondary action-button" type="button">更新密码</button>', 'settings-password') +
              operationPanel('端口配置', service.current_addr || '当前监听端口', '◎', '<button class="secondary action-button" type="button">保存端口</button>', 'settings-port') +
              operationPanel('HTTPS 证书', '到期 ' + (service.cert_not_after ? new Date(service.cert_not_after).toLocaleString() : '未配置'), 'TLS', '<button class="secondary action-button" type="button">上传证书</button>', 'settings-certificate') +
              operationPanel('配置引导', '重新打开端口、密码和证书配置流程', 'CFG', '<button class="secondary action-button" type="button">打开引导</button>', '') +
            '</div>' +
            '<div class="settings-column settings-column-operations">' +
              settingsSectionHead('维护与发布', '数据库、在线更新和版本信息') +
              operationPanel('数据库下载', '数据库大小 ' + fmtBytes(data.database_size_bytes || 0) + ' · ' + fmtStoredDays(data.metric_stored_days, data.retention_hours || 0) + ' · ' + fmtBytes(disk.free_bytes) + ' 空闲', 'DB', '<a class="secondary action-button" href="/api/v1/admin/database/download" download>下载数据库</a>', 'settings-database') +
              updatePanel() +
              projectPanel() +
            '</div>' +
          '</section></div>';
      hydrateUpdatePanel();
      hydrateProjectPanel();
    }
    function settingsSectionHead(title, caption) {
      return '<div class="settings-section-head"><div><h2>' + esc(title) + '</h2><p>' + esc(caption) + '</p></div></div>';
    }
    function settingStat(label, value, caption) {
      return '<div class="setting-stat" data-testid="setting-stat"><span>' + esc(label) + '</span><strong>' + esc(value) + '</strong><p>' + esc(caption) + '</p></div>';
    }
    function operationPanel(title, caption, icon, action, testID) {
      return '<article class="panel setting-operation"' + (testID ? ' data-testid="' + esc(testID) + '"' : '') + '><div class="operation-head"><div class="operation-icon">' + esc(icon) + '</div><div><h2>' + esc(title) + '</h2><p>' + esc(caption) + '</p></div></div>' + action + '</article>';
    }
    function updatePanel() {
      const service = state.data && state.data.service || {};
      return '<article class="panel setting-operation update-card" data-testid="settings-update"><div class="operation-head"><div class="operation-icon">UP</div><div><h2>在线更新</h2><p>检查 Git 上游版本</p></div><span class="pill warn" id="updateState">未检查</span></div><div class="update-compare"><div><span>当前提交</span><strong id="updateLocal">-</strong></div><div><span>远端提交</span><strong id="updateRemote">-</strong></div><div><span>落后</span><strong id="updateBehind">0</strong></div></div><div class="update-meta"><div><span>运行版本</span><strong id="updateRunningVersion">-</strong></div><div><span>仓库版本</span><strong id="updateRepoVersion">-</strong></div><div><span>检查时间</span><strong id="updateChecked">-</strong></div></div><form class="settings-form inline update-proxy-form" onsubmit="saveUpdateProxy(event)"><label>更新代理<input id="updateProxyInput" value="' + esc(service.update_proxy || '') + '" placeholder="http://127.0.0.1:7890"></label><button class="secondary" type="submit">保存代理</button></form><p class="update-note" id="updateProxyMessage"></p><div class="settings-button-row"><button class="secondary" type="button" onclick="checkUpdateStatus()">检查更新</button><button class="primary narrow" type="button" id="updateApplyButton" onclick="applyUpdate()" disabled>更新</button></div><div class="update-progress hidden" id="updateProgress"></div><div class="update-note-row notice update-note" id="updateMessageRow"><p><span id="updateMessage">服务端会先检查依赖，再拉取、构建并自动重启。</span><button class="icon-button inline-help hidden" type="button" id="updateDetailButton" onclick="showUpdateDetail()" title="查看 Git 原始错误">?</button></p></div></article>';
    }
    async function saveUpdateProxy(event) {
      event.preventDefault();
      const input = document.getElementById('updateProxyInput');
      const message = document.getElementById('updateProxyMessage');
      try {
        const result = await api('/api/v1/admin/update/proxy', {method: 'POST', body: JSON.stringify({proxy_url: input ? input.value.trim() : ''})});
        if (state.data && result.service) state.data.service = result.service;
        if (message) {
          message.className = 'notice update-note';
          message.textContent = result.service && result.service.update_proxy ? '更新代理已保存' : '更新代理已清空';
        }
        checkUpdateStatus(true);
      } catch (err) {
        if (message) {
          message.className = 'error update-note';
          message.textContent = err.message;
        }
      }
    }
    function readCachedUpdateStatus() {
      try {
        const cached = JSON.parse(localStorage.getItem(updateStatusCacheKey) || 'null');
        if (!cached || !cached.status || !cached.cached_at) return null;
        return cached;
      } catch (_) {
        return null;
      }
    }
    function storeCachedUpdateStatus(status) {
      localStorage.setItem(updateStatusCacheKey, JSON.stringify({status, cached_at: new Date().toISOString()}));
    }
    function hydrateUpdatePanel() {
      const cached = readCachedUpdateStatus();
      const cachedAt = cached ? new Date(cached.cached_at).getTime() : 0;
      if (cached && Date.now() - cachedAt < updateStatusCacheTTL) {
        renderUpdateStatus(cached.status);
      } else {
        checkUpdateStatus(true);
      }
      if (!state.updateCheckTimer) {
        state.updateCheckTimer = setInterval(() => {
          if (state.view === 'settings') checkUpdateStatus(true);
        }, updateStatusCacheTTL);
      }
    }
    async function checkUpdateStatus(silent) {
      const state = document.getElementById('updateState');
      const message = document.getElementById('updateMessage');
      if (!silent && state) state.textContent = '检查中';
      if (!silent && message) message.textContent = '正在读取 Git 状态';
      try {
        const status = await api('/api/v1/admin/update/status');
        storeCachedUpdateStatus(status);
        renderUpdateStatus(status);
      } catch (err) {
        if (!silent) renderUpdateError(err.message || 'update status failed');
      }
    }
    async function applyUpdate() {
      if (!window.confirm('确认更新服务端？服务端会构建、拉取并自动重启。')) return;
      const button = document.getElementById('updateApplyButton');
      const message = document.getElementById('updateMessage');
      if (button) button.disabled = true;
      if (message) message.textContent = '正在检查依赖、拉取并构建更新';
      renderUpdateProgress(1);
      const timer = setTimeout(() => renderUpdateProgress(2), 1200);
      try {
        const result = await api('/api/v1/admin/update/apply', {method: 'POST'});
        clearTimeout(timer);
        if (result.status) storeCachedUpdateStatus(result.status);
        renderUpdateStatus(result.status || {});
        renderUpdateProgress(result.restarting ? 4 : 5);
        if (message) message.textContent = result.restarting ? '更新已构建完成，服务端正在自动重启' : (result.restart_required ? '更新已拉取并构建完成，正在等待服务端重启' : '当前已经是最新版本');
      } catch (err) {
        clearTimeout(timer);
        renderUpdateProgress(0);
        renderUpdateError('在线更新失败，请查看详情并检查服务器网络、Git 上游或更新代理配置。', err.message || 'update failed');
      }
    }
    function renderUpdateProgress(step) {
      const root = document.getElementById('updateProgress');
      if (!root) return;
      if (!step) {
        root.classList.add('hidden');
        root.innerHTML = '';
        return;
      }
      const labels = ['已发送更新请求', '依赖预检、构建远端提交并执行 fast-forward 拉取', '更新已应用，准备自动重启', '服务端正在自动重启', '等待服务端恢复，恢复后自动刷新'];
      root.classList.remove('hidden');
      root.innerHTML = labels.map((label, index) => '<div class="' + (index + 1 < step ? 'done' : index + 1 === step ? 'active' : '') + '"><span>' + (index + 1) + '</span><p>' + esc(label) + '</p></div>').join('');
    }
    function renderUpdateStatus(status) {
      const local = document.getElementById('updateLocal');
      const remote = document.getElementById('updateRemote');
      const behind = document.getElementById('updateBehind');
      const checked = document.getElementById('updateChecked');
      const runningVersion = document.getElementById('updateRunningVersion');
      const repoVersion = document.getElementById('updateRepoVersion');
      if (local) local.textContent = shortHash(status.local_commit);
      if (remote) remote.textContent = shortHash(status.remote_commit);
      if (behind) behind.textContent = String(status.behind || 0);
      if (runningVersion) runningVersion.textContent = status.running_version ? 'v' + status.running_version : '-';
      if (repoVersion) repoVersion.textContent = status.repo_version ? 'v' + status.repo_version : '-';
      if (checked) checked.textContent = status.checked_at ? new Date(status.checked_at).toLocaleString() : '-';
      const viewState = updateState(status);
      const stateNode = document.getElementById('updateState');
      const message = document.getElementById('updateMessage');
      const messageRow = document.getElementById('updateMessageRow');
      const detailButton = document.getElementById('updateDetailButton');
      const button = document.getElementById('updateApplyButton');
      state.updateDetail = status.detail || '';
      if (stateNode) {
        stateNode.className = 'pill ' + viewState.tone;
        stateNode.textContent = viewState.label;
      }
      if (message) {
        message.textContent = viewState.message;
      }
      if (messageRow) messageRow.className = 'update-note-row update-note ' + (viewState.tone === 'bad' ? 'error' : 'notice');
      if (detailButton) detailButton.classList.toggle('hidden', !state.updateDetail);
      if (button) {
        button.disabled = !(status.supported && status.upstream && (status.available || status.binary_outdated) && !status.dirty && !status.ahead);
        button.textContent = '更新';
      }
    }
    function renderUpdateError(message, detail) {
      const stateNode = document.getElementById('updateState');
      const messageNode = document.getElementById('updateMessage');
      const messageRow = document.getElementById('updateMessageRow');
      const detailButton = document.getElementById('updateDetailButton');
      const button = document.getElementById('updateApplyButton');
      state.updateDetail = detail || message || '';
      if (stateNode) {
        stateNode.className = 'pill bad';
        stateNode.textContent = '失败';
      }
      if (messageNode) {
        messageNode.textContent = message;
      }
      if (messageRow) messageRow.className = 'update-note-row update-note error';
      if (detailButton) detailButton.classList.toggle('hidden', !state.updateDetail);
      if (button) button.disabled = true;
    }
    function showUpdateDetail() {
      window.alert(state.updateDetail || '没有可用的 Git 原始错误');
    }
    function updateState(status) {
      if (!status || !status.supported) return {label: '不可用', tone: 'bad', message: (status && status.message) || '服务端未运行在 Git 工作区'};
      if (status.failed) return {label: '检查失败', tone: 'bad', message: status.message || '检查 Git 上游失败'};
      if (status.dirty) return {label: '已阻止', tone: 'bad', message: '服务端工作区存在未提交改动，已阻止自动拉取'};
      if (!status.upstream) return {label: '未绑定', tone: 'warn', message: status.message || '当前分支没有 Git upstream'};
      if (status.ahead > 0 && status.behind > 0) return {label: '分叉', tone: 'bad', message: '本地和上游存在分叉，不能自动 fast-forward'};
      if (status.ahead > 0) return {label: '本地超前', tone: 'warn', message: '本地提交超前上游，面板不会执行拉取'};
      if (status.available) return {label: '有新版本', tone: 'good', message: String(status.behind || 0) + ' 个提交可拉取、构建并自动重启'};
      if (status.binary_outdated) return {label: '需重建', tone: 'warn', message: '运行中的服务端二进制与当前仓库不一致，可重建并自动重启'};
      return {label: '最新', tone: 'good', message: status.message || '已经是最新版本'};
    }
    function shortHash(value) {
      value = String(value || '');
      return value ? value.slice(0, 12) : '-';
    }
    function projectPanel() {
      const logo = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256"><defs><linearGradient id="projectShell" x1="28" y1="24" x2="222" y2="228" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#146C78"/><stop offset=".58" stop-color="#198754"/><stop offset="1" stop-color="#B26A00"/></linearGradient><linearGradient id="projectChip" x1="78" y1="73" x2="178" y2="181" gradientUnits="userSpaceOnUse"><stop offset="0" stop-color="#FFFFFF"/><stop offset="1" stop-color="#DFF3F2"/></linearGradient></defs><rect x="18" y="18" width="220" height="220" rx="46" fill="url(#projectShell)"/><path d="M62 89h34M62 166h34M160 62v34M160 160v34M178 128h32" fill="none" stroke="#E8FBF6" stroke-width="11" stroke-linecap="round" stroke-linejoin="round"/><g fill="#E8FBF6"><circle cx="62" cy="89" r="13"/><circle cx="62" cy="166" r="13"/><circle cx="160" cy="62" r="13"/><circle cx="160" cy="194" r="13"/><circle cx="210" cy="128" r="13"/></g><rect x="84" y="84" width="88" height="88" rx="22" fill="url(#projectChip)"/><rect x="105" y="105" width="46" height="46" rx="12" fill="#146C78"/><path d="M118 130h16c8 0 13-5 13-13s-5-13-13-13h-16v47M121 130h23" fill="none" stroke="#F7FFFC" stroke-width="9" stroke-linecap="round" stroke-linejoin="round"/></svg>';
      return '<article class="panel setting-operation project-card release-card" data-testid="settings-project"><div class="operation-head"><div class="operation-icon project-logo">' + logo + '</div><div><h2>版本与变更</h2><p id="releaseSummary">GPUFleet 发布信息</p></div></div><div class="project-meta"><div><span>作者</span><strong id="releaseAuthor">stlin256</strong></div><div><span>版本</span><strong id="releaseVersion">-</strong></div><div><span>提交</span><strong id="releaseCommit">dev</strong></div><div><span>构建时间</span><strong id="releaseBuild">-</strong></div><div class="project-url"><span>仓库地址</span><a id="releaseRepository" href="https://github.com/stlin256/GPU-Fleet" target="_blank" rel="noreferrer">https://github.com/stlin256/GPU-Fleet</a></div></div><div class="changelog-panel" data-testid="settings-changelog"><div class="changelog-head"><span>最近变更</span></div><div id="releaseChangelog"><p>正在读取版本信息</p></div></div><a class="secondary action-button" id="releaseRepositoryButton" href="https://github.com/stlin256/GPU-Fleet" target="_blank" rel="noreferrer">打开 GitHub</a></article>';
    }
    async function hydrateProjectPanel() {
      try {
        renderProjectInfo(await api('/api/v1/version'));
      } catch (err) {
        const root = document.getElementById('releaseChangelog');
        if (root) root.innerHTML = '<p>' + esc(err.message || '读取版本信息失败') + '</p>';
      }
    }
    function renderProjectInfo(release) {
      const versionText = release && release.version ? 'v' + release.version : '-';
      const repository = release && release.repository || 'https://github.com/stlin256/GPU-Fleet';
      const summary = document.getElementById('releaseSummary');
      const author = document.getElementById('releaseAuthor');
      const version = document.getElementById('releaseVersion');
      const commit = document.getElementById('releaseCommit');
      const build = document.getElementById('releaseBuild');
      const repositoryLink = document.getElementById('releaseRepository');
      const repositoryButton = document.getElementById('releaseRepositoryButton');
      if (summary) summary.textContent = (release && release.product || 'GPUFleet') + ' ' + versionText;
      if (author) author.textContent = release && release.author || 'stlin256';
      if (version) version.textContent = versionText;
      if (commit) commit.textContent = release && release.commit && release.commit !== 'dev' ? release.commit : 'dev';
      if (build) build.textContent = release && release.build_time ? new Date(release.build_time).toLocaleString() : '-';
      if (repositoryLink) {
        repositoryLink.href = repository;
        repositoryLink.textContent = repository;
      }
      if (repositoryButton) repositoryButton.href = repository;
      const root = document.getElementById('releaseChangelog');
      if (!root) return;
      const latest = release && release.changelog && release.changelog[0];
      root.innerHTML = latest ? renderChangelogEntry(latest) : '<p>暂无变更记录</p>';
    }
    function renderChangelogEntry(entry) {
      return '<div class="changelog-entry"><div><strong>v' + esc(entry.version || '-') + '</strong><span>' + esc(entry.date || '-') + '</span></div><p>' + esc(entry.title || entry.title_en || '-') + '</p>' + renderChangelogList('新增', entry.added) + renderChangelogList('变更', entry.changed) + renderChangelogList('安全', entry.security) + renderChangelogList('修复', entry.fixed) + '</div>';
    }
    function renderChangelogList(label, items) {
      if (!items || !items.length) return '';
      return '<div class="changelog-group"><span>' + esc(label) + '</span><ul>' + items.map((item) => '<li>' + esc(item) + '</li>').join('') + '</ul></div>';
    }

    function gpuHealth(item, device) {
      const gpu = item.gpu || {};
      if (!device || !device.enabled || device.status === 'offline') return {tone: 'offline', label: '离线'};
      if (gpu.collection_error) return {tone: 'bad', label: '采集异常'};
      if ((gpu.temperature_celsius || 0) >= 85) return {tone: 'bad', label: '高温'};
      if ((gpu.temperature_celsius || 0) >= 80 || memoryUsage(gpu) >= 90) return {tone: 'warn', label: '关注'};
      return {tone: 'good', label: '正常'};
    }
    function memoryUsage(gpu) {
      return gpu.memory_total_bytes ? gpu.memory_used_bytes / gpu.memory_total_bytes * 100 : 0;
    }
    function metricTone(value, warnAt, badAt) {
      if (typeof value !== 'number' || !Number.isFinite(value)) return 'accent';
      if (value >= badAt) return 'bad';
      if (value >= warnAt) return 'warn';
      return 'good';
    }
    function maxOf(values, fallback) {
      return Math.max(fallback, ...values.map((sample) => sample && sample.value).filter((v) => typeof v === 'number' && Number.isFinite(v)));
    }
    function deviceBorderColor(deviceID) {
      let hash = 0;
      for (let index = 0; index < String(deviceID || '').length; index += 1) {
        hash = ((hash << 5) - hash + String(deviceID).charCodeAt(index)) | 0;
      }
      return deviceBorderPalette[Math.abs(hash) % deviceBorderPalette.length];
    }
    function deviceName(device, fallback) {
      return device && (device.alias || device.hostname) || fallback;
    }
    function shortGPUName(name) {
      return String(name || '').replace(/^NVIDIA\s+/i, '').replace(/^GeForce\s+/i, '');
    }
    function pcieLabel(gpu) {
      const current = [gpu.pcie_link_generation ? 'Gen ' + gpu.pcie_link_generation : '', gpu.pcie_link_width ? 'x' + gpu.pcie_link_width : ''].filter(Boolean).join(' ');
      return current || 'PCIe -';
    }
    function tempToneText(value) {
      if (typeof value !== 'number') return '-';
      if (value >= 85) return '过热';
      if (value >= 80) return '偏高';
      return '正常';
    }
    function timeAgo(value) {
      const delta = Date.now() - new Date(value).getTime();
      if (!Number.isFinite(delta) || delta < 0) return fallbackI18n.language === 'en-US' ? 'just now' : '刚刚';
      const seconds = Math.floor(delta / 1000);
      if (seconds < 60) return fallbackI18n.language === 'en-US' ? seconds + 's ago' : seconds + 's 前';
      const minutes = Math.floor(seconds / 60);
      if (minutes < 60) return fallbackI18n.language === 'en-US' ? minutes + 'm ago' : minutes + 'm 前';
      const hours = Math.floor(minutes / 60);
      return fallbackI18n.language === 'en-US' ? hours + 'h ago' : hours + 'h 前';
    }

    refresh();
    setInterval(refresh, 10000);
  </script>
</body>
</html>`
