import React, { useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider, useQuery, useQueryClient } from '@tanstack/react-query';
import * as echarts from 'echarts/core';
import { BarChart, LineChart } from 'echarts/charts';
import { GridComponent, TooltipComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import {
  Activity,
  CheckCircle2,
  Clipboard,
  Cpu,
  Database,
  Download,
  FileKey2,
  Gauge,
  HardDrive,
  KeyRound,
  LockKeyhole,
  LogIn,
  LogOut,
  MonitorUp,
  Moon,
  Network,
  Plus,
  Power,
  PowerOff,
  RefreshCw,
  Save,
  Server,
  Settings,
  Sun,
  Upload
} from 'lucide-react';
import {
  applyInitialSetup,
  applySetup,
  changePassword,
  createDevice,
  databaseDownloadURL,
  Device,
  getGPUSeries,
  getOverview,
  getSetupStatus,
  getStats,
  GPUSeriesPoint,
  GPUStats,
  login,
  logout,
  Overview,
  reopenSetup,
  rotateDeviceSecret,
  ServiceStatus,
  setDeviceEnabled,
  SetupStatus,
  StoredGPU,
  StoredProcess,
  updateServerConfig,
  uploadCertificate
} from './api';
import './styles.css';

echarts.use([BarChart, LineChart, GridComponent, TooltipComponent, CanvasRenderer]);

const queryClient = new QueryClient();
type View = 'overview' | 'devices' | 'gpus' | 'settings';
type AuthState = 'checking' | 'setup' | 'authenticated' | 'anonymous';
type Theme = 'light' | 'dark';
type TrendTone = 'good' | 'warn' | 'bad' | 'accent';

const deviceBorderPalette = ['#146c78', '#6750a4', '#b26a00', '#198754', '#c54040', '#2f6fbd', '#8a5a00', '#00806a'];

function initialTheme(): Theme {
  const stored = window.localStorage.getItem('gpufleet-theme');
  if (stored === 'light' || stored === 'dark') return stored;
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function fmtBytes(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB'];
  let size = value;
  let index = 0;
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024;
    index += 1;
  }
  return `${size.toFixed(index === 0 ? 0 : 1)} ${units[index]}`;
}

function pct(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-';
  return `${Math.round(value)}%`;
}

function watts(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-';
  return `${value.toFixed(1)} W`;
}

function mhz(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-';
  return `${Math.round(value)} MHz`;
}

function temp(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value)) return '-';
  return `${Math.round(value)}°C`;
}

function fmtHours(value?: number) {
  if (typeof value !== 'number' || Number.isNaN(value) || value <= 0) return '-';
  if (value % 24 === 0) return `${Math.round(value / 24)} 天`;
  if (value > 24) return `${Math.floor(value / 24)} 天 ${value % 24} 小时`;
  return `${value} 小时`;
}

function fmtDateTime(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleString();
}

function portFromLocation() {
  const parsed = Number(window.location.port || (window.location.protocol === 'https:' ? 443 : 80));
  return Number.isInteger(parsed) && parsed > 0 ? parsed : 8080;
}

function serviceFromOverview(data?: Overview): SetupStatus | undefined {
  if (!data?.service) return undefined;
  return {
    setup_required: false,
    setup_complete: data.setup_complete,
    service: data.service
  };
}

function App() {
  const [authState, setAuthState] = useState<AuthState>('checking');
  const [setupStatus, setSetupStatus] = useState<SetupStatus>();
  const [theme, setTheme] = useState<Theme>(initialTheme);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
    window.localStorage.setItem('gpufleet-theme', theme);
  }, [theme]);

  function toggleTheme() {
    setTheme((current) => current === 'dark' ? 'light' : 'dark');
  }

  useEffect(() => {
    let cancelled = false;
    getSetupStatus()
      .then((status) => {
        if (cancelled) return;
        setSetupStatus(status);
        if (status.setup_required) {
          setAuthState('setup');
          return;
        }
        getOverview()
          .then(() => {
            if (!cancelled) setAuthState('authenticated');
          })
          .catch(() => {
            if (!cancelled) setAuthState('anonymous');
          });
      })
      .catch(() => {
        if (!cancelled) setAuthState('anonymous');
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (authState === 'checking') {
    return <LoadingScreen theme={theme} onToggleTheme={toggleTheme} />;
  }
  if (authState === 'setup') {
    return (
      <SetupWizard
        mode="initial"
        status={setupStatus}
        theme={theme}
        onToggleTheme={toggleTheme}
        onComplete={() => setAuthState('authenticated')}
      />
    );
  }
  if (authState === 'anonymous') {
    return <Login onSuccess={() => setAuthState('authenticated')} theme={theme} onToggleTheme={toggleTheme} />;
  }
  return <Dashboard onUnauthorized={() => setAuthState('anonymous')} theme={theme} onToggleTheme={toggleTheme} />;
}

function LoadingScreen({ theme, onToggleTheme }: { theme: Theme; onToggleTheme: () => void }) {
  return (
    <main className="login-shell">
      <div className="login-panel auth-loading">
        <div className="login-head">
          <div className="brand">
            <span className="brand-mark">G</span>
            <span>GPUFleet</span>
          </div>
          <ThemeToggle theme={theme} onToggle={onToggleTheme} />
        </div>
        <h1>正在连接</h1>
        <p>检查当前 Web 会话</p>
      </div>
    </main>
  );
}

function SetupWizard({
  mode,
  status,
  theme,
  onToggleTheme,
  onComplete,
  onCancel
}: {
  mode: 'initial' | 'authenticated';
  status?: SetupStatus;
  theme: Theme;
  onToggleTheme: () => void;
  onComplete: () => void;
  onCancel?: () => void;
}) {
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [port, setPort] = useState(String(status?.service.configured_port || portFromLocation()));
  const [certificatePEM, setCertificatePEM] = useState('');
  const [privateKeyPEM, setPrivateKeyPEM] = useState('');
  const [certificateName, setCertificateName] = useState('未选择');
  const [keyName, setKeyName] = useState('未选择');
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const service = status?.service;
  const requirePassword = mode === 'initial';

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setError('');
    setMessage('');
    const parsedPort = Number(port);
    if (!Number.isInteger(parsedPort) || parsedPort < 1 || parsedPort > 65535) {
      setError('端口范围应为 1-65535');
      return;
    }
    if ((requirePassword || password || confirmPassword) && password.length < 8) {
      setError('密码至少 8 位');
      return;
    }
    if (password !== confirmPassword) {
      setError('两次密码不一致');
      return;
    }
    if ((certificatePEM && !privateKeyPEM) || (!certificatePEM && privateKeyPEM)) {
      setError('证书和私钥需要同时上传');
      return;
    }
    setLoading(true);
    try {
      const payload = {
        password: password || undefined,
        port: parsedPort,
        certificate_pem: certificatePEM || undefined,
        private_key_pem: privateKeyPEM || undefined
      };
      const result = mode === 'initial' ? await applyInitialSetup(payload) : await applySetup(payload);
      if (mode === 'initial' && password) {
        await login(password);
      }
      setMessage(result.restart_required ? '配置已保存，重启服务后端口或 HTTPS 生效' : '配置已保存');
      onComplete();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'setup failed');
    } finally {
      setLoading(false);
    }
  }

  async function loadPEM(event: React.ChangeEvent<HTMLInputElement>, target: 'cert' | 'key') {
    const file = event.target.files?.[0];
    if (!file) return;
    const text = await file.text();
    if (target === 'cert') {
      setCertificatePEM(text);
      setCertificateName(file.name);
    } else {
      setPrivateKeyPEM(text);
      setKeyName(file.name);
    }
  }

  return (
    <main className={mode === 'initial' ? 'login-shell' : 'setup-inline'} data-testid={mode === 'initial' ? 'setup-wizard' : 'setup-wizard-inline'}>
      <form className={`setup-panel ${mode === 'initial' ? 'panel' : ''}`} onSubmit={submit}>
        <div className="login-head">
          <div className="brand">
            <span className="brand-mark">G</span>
            <span>GPUFleet</span>
          </div>
          <ThemeToggle theme={theme} onToggle={onToggleTheme} />
        </div>
        <div className="setup-title">
          <span className="pill good">{service?.current_scheme?.toUpperCase() ?? 'HTTP'}</span>
          <h1>{mode === 'initial' ? '首次配置' : '配置引导'}</h1>
          <p>{service ? `${service.current_addr} · ${service.current_scheme.toUpperCase()}` : '初始化服务访问参数'}</p>
        </div>

        <div className="setup-grid">
          <label>
            {mode === 'initial' ? '访问密码' : '新密码'}
            <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="new-password" placeholder={mode === 'initial' ? '至少 8 位' : '留空则不变'} />
          </label>
          <label>
            确认密码
            <input value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} type="password" autoComplete="new-password" placeholder={mode === 'initial' ? '再次输入密码' : '仅修改密码时填写'} />
          </label>
          <label>
            访问端口
            <input value={port} onChange={(event) => setPort(event.target.value)} type="number" min={1} max={65535} inputMode="numeric" />
          </label>
          <div className="setup-file-row">
            <label>
              HTTPS 证书
              <input type="file" accept=".pem,.crt,.cer" onChange={(event) => loadPEM(event, 'cert')} />
              <span>{certificateName}</span>
            </label>
            <label>
              私钥文件
              <input type="file" accept=".pem,.key" onChange={(event) => loadPEM(event, 'key')} />
              <span>{keyName}</span>
            </label>
          </div>
        </div>

        <div className="setup-actions">
          {onCancel && <button className="secondary" type="button" onClick={onCancel}>取消</button>}
          <button className="primary compact" disabled={loading}>
            <Save size={17} />
            {loading ? '保存中' : '保存配置'}
          </button>
        </div>
        {service?.cert_not_after && <p className="notice">当前证书到期：{fmtDateTime(service.cert_not_after)}</p>}
        {message && <p className="notice">{message}</p>}
        {error && <p className="error">{error}</p>}
      </form>
    </main>
  );
}

function Login({ onSuccess, theme, onToggleTheme }: { onSuccess: () => void; theme: Theme; onToggleTheme: () => void }) {
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError('');
    try {
      await login(password);
      onSuccess();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'login failed');
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="login-shell">
      <form className="login-panel" onSubmit={submit}>
        <div className="login-head">
          <div className="brand">
            <span className="brand-mark">G</span>
            <span>GPUFleet</span>
          </div>
          <ThemeToggle theme={theme} onToggle={onToggleTheme} />
        </div>
        <h1>登录面板</h1>
        <label>
          密码
          <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="current-password" />
        </label>
        <button className="primary" disabled={loading}>
          <LogIn size={18} />
          {loading ? '登录中' : '登录'}
        </button>
        {error && <p className="error">{error}</p>}
      </form>
    </main>
  );
}

function Dashboard({ onUnauthorized, theme, onToggleTheme }: { onUnauthorized: () => void; theme: Theme; onToggleTheme: () => void }) {
  const [view, setView] = useState<View>('overview');
  const overview = useQuery({
    queryKey: ['overview'],
    queryFn: getOverview,
    refetchInterval: 10000,
    retry: false
  });
  const stats = useQuery({
    queryKey: ['stats', 24],
    queryFn: () => getStats(24),
    enabled: overview.isSuccess,
    refetchInterval: 30000
  });

  useEffect(() => {
    if (overview.error instanceof Error && overview.error.message.includes('login')) {
      onUnauthorized();
    }
  }, [overview.error, onUnauthorized]);

  const data = overview.data;
  const statRows = stats.data?.stats ?? [];
  const memoryPct = data?.memory_total_bytes ? (data.memory_used_bytes / data.memory_total_bytes) * 100 : 0;
  const titles: Record<View, string> = {
    overview: 'GPU 资源总览',
    devices: '设备管理',
    gpus: 'GPU 监控',
    settings: '服务设置'
  };

  async function signOut() {
    await logout().catch(() => undefined);
    onUnauthorized();
  }

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">G</span>
          <span>GPUFleet</span>
        </div>
        <nav>
          <button className={view === 'overview' ? 'active' : ''} onClick={() => setView('overview')}><Activity size={17} />总览</button>
          <button className={view === 'devices' ? 'active' : ''} onClick={() => setView('devices')}><Server size={17} />设备</button>
          <button className={view === 'gpus' ? 'active' : ''} onClick={() => setView('gpus')}><Cpu size={17} />GPU</button>
          <button className={view === 'settings' ? 'active' : ''} onClick={() => setView('settings')}><Settings size={17} />设置</button>
        </nav>
      </aside>
      <main className="content">
        <header className="topbar">
          <div>
            <h1>{titles[view]}</h1>
            <p>{data ? `服务端时间 ${new Date(data.server_time).toLocaleString()}` : '等待服务端数据'}</p>
          </div>
          <div className="top-actions">
            <ThemeToggle theme={theme} onToggle={onToggleTheme} />
            <button className="icon-button" onClick={() => overview.refetch()} title="刷新">
              <RefreshCw size={18} />
            </button>
            <button className="icon-button" onClick={signOut} title="退出登录">
              <LogOut size={18} />
            </button>
          </div>
        </header>

        {data?.disk.status === 'critical' && <div className="banner danger">磁盘空间低于保护阈值，服务端已拒绝新指标写入。</div>}
        {overview.error && <div className="banner danger">{overview.error.message}</div>}

        {view === 'overview' && <OverviewPage data={data} statRows={statRows} theme={theme} />}
        {view === 'gpus' && <GPUDetailPage data={data} statRows={statRows} memoryPct={memoryPct} theme={theme} />}

        {view === 'devices' && <DeviceAdminPanel data={data} />}
        {view === 'settings' && <SettingsPanel data={data} theme={theme} onToggleTheme={onToggleTheme} />}
      </main>
    </div>
  );
}

function ThemeToggle({ theme, onToggle }: { theme: Theme; onToggle: () => void }) {
  return (
    <button className="icon-button theme-toggle" onClick={onToggle} title={theme === 'dark' ? '切换浅色' : '切换深色'} data-testid="theme-toggle">
      {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
    </button>
  );
}

function OverviewPage({ data, statRows, theme }: { data?: Overview; statRows: GPUStats[]; theme: Theme }) {
  const gpus = data?.latest_gpus ?? [];
  const devices = data?.devices ?? [];
  const memoryPct = data?.memory_total_bytes ? (data.memory_used_bytes / data.memory_total_bytes) * 100 : 0;
  const hotCount = gpus.filter((item) => (item.gpu.temperature_celsius ?? 0) >= 80).length;
  const busyCount = gpus.filter((item) => (item.gpu.utilization_gpu_percent ?? 0) >= 80).length;

  return (
    <>
      <section className="fleet-command">
        <div className="fleet-command-copy">
          <span className="fleet-eyebrow">Fleet Live</span>
          <h2>多机 GPU 运行态</h2>
          <p>{devices.length > 0 ? `${devices.length} 台设备，${gpus.length} 块 GPU，按最新上报状态汇总。` : '等待客户端上报 GPU 运行信息。'}</p>
        </div>
        <div className="fleet-kpis">
          <FleetKPI label="在线设备" value={`${data?.online_device_count ?? 0}/${data?.device_count ?? 0}`} tone={(data?.online_device_count ?? 0) === (data?.device_count ?? 0) ? 'good' : 'warn'} />
          <FleetKPI label="GPU 总数" value={String(data?.gpu_count ?? 0)} />
          <FleetKPI label="忙碌 GPU" value={String(busyCount)} tone={busyCount > 0 ? 'accent' : 'good'} />
          <FleetKPI label="高温 GPU" value={String(hotCount)} tone={hotCount > 0 ? 'bad' : 'good'} />
          <FleetKPI label="显存占用" value={pct(memoryPct)} />
          <FleetKPI label="磁盘保护" value={(data?.disk.status ?? 'ok').toUpperCase()} tone={data?.disk.status === 'critical' ? 'bad' : data?.disk.status === 'warning' ? 'warn' : 'good'} />
        </div>
      </section>

      <section className="overview-layout">
        <FleetBoard items={gpus} devices={devices} />
        <div className="overview-side">
          <FleetUtilPanel items={gpus} theme={theme} />
          <DevicePanel data={data} />
        </div>
      </section>

      <section className="overview-secondary">
        <ProcessPanel items={data?.latest_processes ?? []} />
        <StatsPanel statRows={statRows} />
      </section>
    </>
  );
}

function GPUDetailPage({ data, statRows, memoryPct, theme }: { data?: Overview; statRows: GPUStats[]; memoryPct: number; theme: Theme }) {
  return (
    <>
      <section className="stat-grid">
        <Metric icon={<MonitorUp />} label="在线设备" value={`${data?.online_device_count ?? 0} / ${data?.device_count ?? 0}`} />
        <Metric icon={<Cpu />} label="GPU 数量" value={String(data?.gpu_count ?? 0)} />
        <Metric icon={<Gauge />} label="平均利用率" value={pct(data?.average_utilization ?? 0)} />
        <Metric icon={<Database />} label="显存占用" value={pct(memoryPct)} />
        <Metric icon={<HardDrive />} label="磁盘保护" value={(data?.disk.status ?? 'ok').toUpperCase()} tone={data?.disk.status} />
      </section>

      <section className="main-grid">
        <div className="panel">
          <div className="panel-head">
            <h2>GPU 详细状态</h2>
            <span>{data?.latest_gpus.length ?? 0}</span>
          </div>
          <div className="gpu-grid">
            {(data?.latest_gpus ?? []).map((item) => <GPUCard key={`${item.device_id}-${item.gpu.gpu_id}`} item={item} />)}
          </div>
          <UtilChart items={data?.latest_gpus ?? []} theme={theme} />
        </div>
        <div className="stack">
          <DevicePanel data={data} />
          <ProcessPanel items={data?.latest_processes ?? []} />
        </div>
      </section>

      <StatsPanel statRows={statRows} />
    </>
  );
}

function FleetKPI({ label, value, tone }: { label: string; value: string; tone?: 'good' | 'warn' | 'bad' | 'accent' }) {
  return (
    <div className={`fleet-kpi ${tone ?? ''}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function FleetBoard({ items, devices }: { items: StoredGPU[]; devices: Device[] }) {
  const deviceMap = useMemo(() => new Map(devices.map((device) => [device.id, device])), [devices]);
  const cards = items.map((item) => ({ item, device: deviceMap.get(item.device_id), health: gpuHealth(item, deviceMap.get(item.device_id)) }));

  return (
    <section className="fleet-board panel" data-testid="fleet-board">
      <div className="panel-head fleet-board-head">
        <div>
          <h2>GPU Fleet</h2>
          <p>卡片化查看多设备 GPU 运行状态</p>
        </div>
        <span>{items.length} GPUs</span>
      </div>
      <div className="fleet-card-grid">
        {cards.map(({ item, device, health }) => (
          <FleetGPUCard item={item} device={device} health={health} key={`${item.device_id}-${item.gpu.gpu_id}`} />
        ))}
        {cards.length === 0 && <p className="empty">暂无 GPU 上报</p>}
      </div>
    </section>
  );
}

function FleetGPUCard({ item, device, health }: { item: StoredGPU; device?: Device; health: ReturnType<typeof gpuHealth> }) {
  const gpu = item.gpu;
  const util = gpu.utilization_gpu_percent;
  const mem = memoryUsagePercent(item);
  const powerLimit = gpu.power_limit_watts ?? gpu.power_enforced_limit_watts;
  const deviceColor = deviceBorderColor(item.device_id);
  const series = useQuery({
    queryKey: ['gpu-series', item.device_id, gpu.gpu_id, 1],
    queryFn: () => getGPUSeries(item.device_id, gpu.gpu_id, 1),
    refetchInterval: 30000,
    retry: false
  });
  const points = series.data ?? [];

  return (
    <article
      className={`fleet-gpu-card ${health.tone}`}
      data-testid="fleet-gpu-card"
      data-device-id={item.device_id}
      data-device-color={deviceColor}
      style={{ '--device-color': deviceColor } as React.CSSProperties}
    >
      {health.tone === 'offline' && <div className="offline-mask">离线</div>}
      <div className="fleet-card-top">
        <div className="fleet-device-cell">
          <span className={`status-dot ${health.tone}`} />
          <div>
            <strong>{deviceName(device, item.device_id)}</strong>
            <p>{shortGPUName(gpu.name || gpu.gpu_id)} · {gpu.gpu_id} · {timeAgo(item.timestamp)}</p>
          </div>
        </div>
        <span className={`pill ${health.tone}`}>{health.label}</span>
      </div>

      <div className="gpu-card-meta">
        <span>{pcieLabel(item)}</span>
        <span>{gpu.pstate || '-'}</span>
        <span>{gpu.compute_capability ? `Compute ${gpu.compute_capability}` : gpu.driver_model || '-'}</span>
      </div>

      <GPUTrendGrid item={item} points={points} />
    </article>
  );
}

function GPUTrendGrid({ item, points, className = 'gpu-trend-grid' }: { item: StoredGPU; points: GPUSeriesPoint[]; className?: string }) {
  const gpu = item.gpu;
  const util = gpu.utilization_gpu_percent;
  const mem = memoryUsagePercent(item);
  const powerLimit = gpu.power_limit_watts ?? gpu.power_enforced_limit_watts;

  return (
    <div className={className}>
      <TrendTile label="GPU 利用率" value={pct(util)} caption={gpu.sm_clock_mhz ? mhz(gpu.sm_clock_mhz) : '最近 1 小时'} values={points.map((point) => point.utilization_gpu_percent)} max={100} tone={metricTone(util, 70, 92)} formatValue={pct} />
      <TrendTile label="显存" value={`${pct(mem)} · ${fmtBytes(gpu.memory_used_bytes)}`} caption={`总量 ${fmtBytes(gpu.memory_total_bytes)}`} values={points.map((point) => point.memory_total_bytes ? (point.memory_used_bytes / point.memory_total_bytes) * 100 : undefined)} max={100} tone={metricTone(mem, 75, 92)} formatValue={pct} />
      <TrendTile label="温度" value={temp(gpu.temperature_celsius)} caption={tempToneText(gpu.temperature_celsius)} values={points.map((point) => point.temperature_celsius)} max={100} tone={metricTone(gpu.temperature_celsius, 80, 88)} formatValue={temp} />
      <TrendTile label="功耗" value={watts(gpu.power_draw_watts)} caption={powerLimit ? `上限 ${watts(powerLimit)}` : gpu.pstate || '-'} values={points.map((point) => point.power_draw_watts)} max={powerLimit || maxSeries(points.map((point) => point.power_draw_watts), 200)} tone={metricTone(powerLimit && gpu.power_draw_watts ? (gpu.power_draw_watts / powerLimit) * 100 : undefined, 78, 95)} formatValue={watts} />
    </div>
  );
}

function TrendTile({ label, value, caption, values, max, tone, formatValue }: { label: string; value: string; caption: string; values: Array<number | undefined>; max: number; tone: TrendTone; formatValue: (value?: number) => string }) {
  const clean = values.filter((item): item is number => typeof item === 'number' && Number.isFinite(item));
  return (
    <div className={`trend-tile ${tone}`} data-testid="gpu-trend-tile">
      <div className="trend-head">
        <div>
          <span>{label}</span>
          <strong>{value}</strong>
        </div>
        <p>{caption}</p>
      </div>
      <Sparkline values={clean} max={max} label={label} formatValue={formatValue} />
    </div>
  );
}

function FleetUtilPanel({ items, theme }: { items: StoredGPU[]; theme: Theme }) {
  const peak = items.reduce((max, item) => Math.max(max, item.gpu.utilization_gpu_percent ?? 0), 0);
  const idle = items.filter((item) => (item.gpu.utilization_gpu_percent ?? 0) < 10).length;
  return (
    <section className="panel fleet-chart-panel">
      <div className="panel-head">
        <div>
          <h2>利用率分布</h2>
          <p>当前快照</p>
        </div>
        <span>峰值 {pct(peak)}</span>
      </div>
      <UtilChart items={items} theme={theme} compact />
      <div className="rail-facts">
        <div><span>空闲 GPU</span><strong>{idle}</strong></div>
        <div><span>活跃 GPU</span><strong>{Math.max(0, items.length - idle)}</strong></div>
      </div>
    </section>
  );
}

function deviceName(device: Device | undefined, fallback: string) {
  return device?.alias || device?.hostname || fallback;
}

function shortGPUName(name: string) {
  return name.replace(/^NVIDIA\s+/i, '').replace(/^GeForce\s+/i, '');
}

function memoryUsagePercent(item: StoredGPU) {
  return item.gpu.memory_total_bytes ? (item.gpu.memory_used_bytes / item.gpu.memory_total_bytes) * 100 : undefined;
}

function metricTone(value: number | undefined, warnAt: number, badAt: number): TrendTone {
  if (typeof value !== 'number' || Number.isNaN(value)) return 'accent';
  if (value >= badAt) return 'bad';
  if (value >= warnAt) return 'warn';
  return 'good';
}

function maxSeries(values: Array<number | undefined>, fallback: number) {
  const clean = values.filter((item): item is number => typeof item === 'number' && Number.isFinite(item));
  return clean.length ? Math.max(fallback, ...clean) : fallback;
}

function Sparkline({ values, max, label, formatValue }: { values: number[]; max: number; label: string; formatValue: (value?: number) => string }) {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);
  const width = 180;
  const height = 58;
  const pad = 4;
  const clean = values.length > 0 ? values : [0];
  const cappedMax = Math.max(1, max);
  const pointData = clean.map((value, index) => {
    const x = clean.length === 1 ? width - pad : pad + (index / (clean.length - 1)) * (width - pad * 2);
    const y = height - pad - (Math.max(0, Math.min(cappedMax, value)) / cappedMax) * (height - pad * 2);
    return { value, x, y };
  });
  const points = pointData.map((point) => `${point.x.toFixed(1)},${point.y.toFixed(1)}`);
  const line = points.join(' ');
  const area = `${pad},${height - pad} ${line} ${width - pad},${height - pad}`;
  const active = hoverIndex === null ? undefined : pointData[hoverIndex];

  function onPointerMove(event: React.PointerEvent<HTMLDivElement>) {
    const rect = event.currentTarget.getBoundingClientRect();
    const ratio = rect.width > 0 ? (event.clientX - rect.left) / rect.width : 1;
    const index = Math.max(0, Math.min(clean.length - 1, Math.round(ratio * (clean.length - 1))));
    setHoverIndex(index);
  }

  return (
    <div className="sparkline-wrap" onPointerMove={onPointerMove} onPointerLeave={() => setHoverIndex(null)}>
      <svg className="sparkline" viewBox={`0 0 ${width} ${height}`} role="img" aria-label={`${label} 历史趋势图`} preserveAspectRatio="none">
        <polyline className="spark-grid" points={`${pad},${height - pad} ${width - pad},${height - pad}`} />
        <polygon className="spark-area" points={area} />
        <polyline className="spark-line" points={line} />
        {active && (
          <>
            <line className="spark-cursor" x1={active.x} x2={active.x} y1={pad} y2={height - pad} />
            <circle className="spark-point" cx={active.x} cy={active.y} r="3.2" />
          </>
        )}
      </svg>
      {active && (
        <div className="spark-tooltip" data-testid="spark-tooltip" style={{ left: `${(active.x / width) * 100}%` }}>
          <span>{label}</span>
          <strong>{formatValue(active.value)}</strong>
          <small>{hoverIndex! + 1}/{clean.length}</small>
        </div>
      )}
    </div>
  );
}

function deviceBorderColor(deviceID: string) {
  let hash = 0;
  for (let index = 0; index < deviceID.length; index += 1) {
    hash = ((hash << 5) - hash + deviceID.charCodeAt(index)) | 0;
  }
  return deviceBorderPalette[Math.abs(hash) % deviceBorderPalette.length];
}

function pcieLabel(item: StoredGPU) {
  const current = [item.gpu.pcie_link_generation ? `Gen ${item.gpu.pcie_link_generation}` : '', item.gpu.pcie_link_width ? `x${item.gpu.pcie_link_width}` : ''].filter(Boolean).join(' ');
  return current || 'PCIe -';
}

function tempToneText(value?: number) {
  if (typeof value !== 'number') return '-';
  if (value >= 85) return '过热';
  if (value >= 80) return '偏高';
  return '正常';
}

function timeAgo(value: string) {
  const delta = Date.now() - new Date(value).getTime();
  if (!Number.isFinite(delta) || delta < 0) return '刚刚';
  const seconds = Math.floor(delta / 1000);
  if (seconds < 60) return `${seconds}s 前`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m 前`;
  const hours = Math.floor(minutes / 60);
  return `${hours}h 前`;
}

function gpuHealth(item: StoredGPU, device?: Device): { tone: 'good' | 'warn' | 'bad' | 'offline'; label: string } {
  if (!device?.enabled || device.status === 'offline') return { tone: 'offline', label: '离线' };
  if (item.gpu.collection_error) return { tone: 'bad', label: '采集异常' };
  if ((item.gpu.temperature_celsius ?? 0) >= 85) return { tone: 'bad', label: '高温' };
  if ((item.gpu.temperature_celsius ?? 0) >= 80 || (memoryUsagePercent(item) ?? 0) >= 90) return { tone: 'warn', label: '关注' };
  return { tone: 'good', label: '正常' };
}

function Metric({ icon, label, value, tone }: { icon: React.ReactNode; label: string; value: string; tone?: string }) {
  return (
    <article className={`metric ${tone ?? ''}`}>
      <div className="metric-icon">{icon}</div>
      <p>{label}</p>
      <strong>{value}</strong>
    </article>
  );
}

function GPUCard({ item }: { item: StoredGPU }) {
  const gpu = item.gpu;
  const pcie = [gpu.pcie_link_generation ? `Gen ${gpu.pcie_link_generation}` : '', gpu.pcie_link_width ? `x${gpu.pcie_link_width}` : ''].filter(Boolean).join(' ');
  const pcieMax = [gpu.pcie_link_generation_max ? `Gen ${gpu.pcie_link_generation_max}` : '', gpu.pcie_link_width_max ? `x${gpu.pcie_link_width_max}` : ''].filter(Boolean).join(' ');
  const deviceColor = deviceBorderColor(item.device_id);
  const series = useQuery({
    queryKey: ['gpu-series-detail', item.device_id, gpu.gpu_id, 1],
    queryFn: () => getGPUSeries(item.device_id, gpu.gpu_id, 1),
    refetchInterval: 30000,
    retry: false
  });
  const points = series.data ?? [];
  const detailRows = [
    ['显存空闲', fmtBytes(gpu.memory_free_bytes)],
    ['显存保留', fmtBytes(gpu.memory_reserved_bytes)],
    ['显存利用', pct(gpu.utilization_memory_percent)],
    ['温度上限', temp(gpu.temperature_limit_celsius)],
    ['显存温度', temp(gpu.temperature_memory_celsius)],
    ['功耗上限', watts(gpu.power_limit_watts ?? gpu.power_enforced_limit_watts)],
    ['风扇', pct(gpu.fan_speed_percent)],
    ['图形时钟', mhz(gpu.graphics_clock_mhz)],
    ['显存时钟', mhz(gpu.memory_clock_mhz)],
    ['SM 时钟', mhz(gpu.sm_clock_mhz)],
    ['视频时钟', mhz(gpu.video_clock_mhz)],
    ['P-State', gpu.pstate || '-'],
    ['PCIe 当前', pcie || '-'],
    ['PCIe 最大', pcieMax || '-'],
    ['Compute', gpu.compute_capability || gpu.compute_mode || '-'],
    ['显示', [gpu.display_active, gpu.display_attached].filter(Boolean).join(' / ') || '-'],
    ['驱动模型', gpu.driver_model || '-'],
    ['VBIOS', gpu.vbios_version || '-'],
    ['ECC', gpu.ecc_mode_current || '-'],
    ['MIG', gpu.mig_mode_current || '-']
  ].filter(([, value]) => value !== '-');

  return (
    <article className="gpu-card" data-device-id={item.device_id} data-device-color={deviceColor} style={{ '--device-color': deviceColor } as React.CSSProperties}>
      <div className="card-title">
        <div>
          <h3>{gpu.name || gpu.gpu_id}</h3>
          <p>{item.device_id} · {gpu.gpu_id}</p>
        </div>
        <span>{pct(gpu.utilization_gpu_percent)}</span>
      </div>
      <GPUTrendGrid item={item} points={points} className="gpu-detail-trend-grid" />
      <div className="gpu-detail-grid">
        {detailRows.map(([label, value]) => (
          <div key={label}>
            <span>{label}</span>
            <strong>{value}</strong>
          </div>
        ))}
      </div>
      {gpu.clock_throttle_reasons && <p className="gpu-note">Throttle {gpu.clock_throttle_reasons}</p>}
    </article>
  );
}

function DevicePanel({ data }: { data?: Overview }) {
  return (
    <section className="panel">
      <div className="panel-head">
        <h2>设备</h2>
        <span>{data?.devices.length ?? 0}</span>
      </div>
      {(data?.devices ?? []).map((device) => (
        <div className="list-row" key={device.id}>
          <div>
            <strong>{device.alias || device.id}</strong>
            <p>{[device.hostname, device.os, device.agent_version].filter(Boolean).join(' · ') || device.id}</p>
          </div>
          <span className={`pill ${device.enabled ? (device.status ?? 'offline') : 'disabled'}`}>{device.enabled ? (device.status ?? 'offline') : 'disabled'}</span>
        </div>
      ))}
    </section>
  );
}

function DeviceAdminPanel({ data }: { data?: Overview }) {
  const query = useQueryClient();
  const [alias, setAlias] = useState('');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState('');
  const [secret, setSecret] = useState<{ deviceId: string; value: string; title: string }>();

  async function refresh() {
    await query.invalidateQueries({ queryKey: ['overview'] });
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setBusy('create');
    setMessage('');
    try {
      const result = await createDevice(alias.trim());
      setAlias('');
      setSecret({ deviceId: result.device.id, value: result.secret, title: '新设备密钥' });
      setMessage(`已创建设备 ${result.device.alias || result.device.id}`);
      await refresh();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'create device failed');
    } finally {
      setBusy('');
    }
  }

  async function toggle(deviceId: string, enabled: boolean) {
    setBusy(`${enabled ? 'enable' : 'disable'}-${deviceId}`);
    setMessage('');
    try {
      await setDeviceEnabled(deviceId, enabled);
      setMessage(enabled ? '设备已启用' : '设备已禁用');
      await refresh();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'device update failed');
    } finally {
      setBusy('');
    }
  }

  async function rotate(deviceId: string) {
    setBusy(`rotate-${deviceId}`);
    setMessage('');
    try {
      const result = await rotateDeviceSecret(deviceId);
      setSecret({ deviceId, value: result.secret, title: '已轮换密钥' });
      setMessage('设备密钥已轮换');
      await refresh();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'secret rotation failed');
    } finally {
      setBusy('');
    }
  }

  return (
    <div className="device-admin">
      <section className="panel">
        <div className="panel-head">
          <h2>注册设备</h2>
          <span>{data?.devices.length ?? 0}</span>
        </div>
        <form className="device-form" onSubmit={submit}>
          <label>
            设备别名
            <input value={alias} onChange={(event) => setAlias(event.target.value)} placeholder="worker-a100-01" />
          </label>
          <button className="primary narrow" disabled={busy === 'create'}>
            <Plus size={17} />
            {busy === 'create' ? '创建中' : '创建'}
          </button>
        </form>
        {message && <p className={message.includes('failed') || message.includes('error') ? 'error' : 'notice'}>{message}</p>}
        {secret && <SecretBox title={secret.title} deviceId={secret.deviceId} value={secret.value} />}
      </section>

      <section className="panel">
        <div className="panel-head">
          <h2>设备列表</h2>
          <span>{data?.devices.length ?? 0}</span>
        </div>
        <div className="device-table">
          {(data?.devices ?? []).map((device) => (
            <div className="device-row" key={device.id}>
              <div>
                <strong>{device.alias || device.id}</strong>
                <p>{device.id} · {device.hostname || '-'} · {device.agent_version || '-'}</p>
              </div>
              <span className={`pill ${device.enabled ? (device.status ?? 'offline') : 'disabled'}`}>{device.enabled ? (device.status ?? 'offline') : 'disabled'}</span>
              <div className="row-actions">
                <button className="secondary" onClick={() => toggle(device.id, !device.enabled)} disabled={busy.endsWith(device.id)} title={device.enabled ? '禁用设备' : '启用设备'}>
                  {device.enabled ? <PowerOff size={16} /> : <Power size={16} />}
                  {device.enabled ? '禁用' : '启用'}
                </button>
                <button className="secondary" onClick={() => rotate(device.id)} disabled={busy === `rotate-${device.id}`} title="轮换密钥">
                  <KeyRound size={16} />
                  轮换
                </button>
              </div>
            </div>
          ))}
          {(data?.devices ?? []).length === 0 && <p className="empty">暂无设备</p>}
        </div>
      </section>
    </div>
  );
}

function SecretBox({ title, deviceId, value }: { title: string; deviceId: string; value: string }) {
  const [copied, setCopied] = useState(false);
  async function copy() {
    await navigator.clipboard?.writeText(value).catch(() => undefined);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1800);
  }
  return (
    <div className="secret-box">
      <div>
        <strong>{title}</strong>
        <p>{deviceId}</p>
      </div>
      <code>{value}</code>
      <button className="secondary" onClick={copy} title="复制密钥">
        {copied ? <CheckCircle2 size={16} /> : <Clipboard size={16} />}
        {copied ? '已复制' : '复制'}
      </button>
    </div>
  );
}

function ProcessPanel({ items }: { items: StoredProcess[] }) {
  return (
    <section className="panel">
      <div className="panel-head">
        <h2>GPU 进程</h2>
        <span>{items.length}</span>
      </div>
      {items.slice(0, 32).map((item) => (
        <div className="list-row" key={`${item.device_id}-${item.process.gpu_id}-${item.process.pid}`}>
          <div>
            <strong>{item.process.process_name || 'unknown'}</strong>
            <p>PID {item.process.pid} · {item.device_id} · {item.process.gpu_id || '-'}</p>
          </div>
          <span className="pill">{fmtBytes(item.process.used_memory_bytes)}</span>
        </div>
      ))}
      {items.length === 0 && <p className="empty">暂无 GPU 进程快照</p>}
    </section>
  );
}

function StatsPanel({ statRows }: { statRows: GPUStats[] }) {
  return (
    <section className="panel">
      <div className="panel-head">
        <h2>24 小时统计</h2>
        <span>{statRows.length}</span>
      </div>
      <div className="stats-table">
        {statRows.map((row) => (
          <div className="table-row" key={`${row.device_id}-${row.gpu_id}`}>
            <div>
              <strong>{row.gpu_name || row.gpu_id}</strong>
              <p>{row.device_id} · {row.gpu_id} · {row.sample_count} samples</p>
            </div>
            <span>{pct(row.average_utilization_percent)}</span>
            <span>{pct(row.idle_sample_percent)} idle</span>
            <span>{row.peak_temperature_celsius ? `${Math.round(row.peak_temperature_celsius)}°C` : '-'}</span>
            <span>{row.peak_power_draw_watts ? `${row.peak_power_draw_watts.toFixed(1)} W` : '-'}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

function SettingsPanel({ data, theme, onToggleTheme }: { data?: Overview; theme: Theme; onToggleTheme: () => void }) {
  const query = useQueryClient();
  const service = data?.service;
  const min = data?.min_free_space_bytes ?? data?.disk.min_free_bytes ?? 0;
  const [wizardOpen, setWizardOpen] = useState(false);
  const [wizardStatus, setWizardStatus] = useState<SetupStatus>();
  const [message, setMessage] = useState('');

  async function refreshOverview() {
    await query.invalidateQueries({ queryKey: ['overview'] });
  }

  async function openWizard() {
    setMessage('');
    try {
      const result = await reopenSetup();
      setWizardStatus(result.setup);
      setWizardOpen(true);
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'setup reopen failed');
    }
  }

  return (
    <div className="settings-page" data-testid="settings-page">
      <section className="settings-status panel">
        <div className="panel-head settings-head">
          <div>
            <h2>服务状态</h2>
            <p>{service ? `${service.current_addr} · ${service.current_scheme.toUpperCase()}` : '等待服务端配置'}</p>
          </div>
          {service?.restart_required && <span className="pill warn">需要重启</span>}
        </div>
        <div className="settings-kpi-grid">
          <SettingStat label="当前协议" value={(service?.current_scheme ?? 'http').toUpperCase()} caption={service?.https_enabled ? '证书已配置' : '未启用证书'} />
          <SettingStat label="访问端口" value={String(service?.configured_port ?? portFromLocation())} caption={service?.current_addr ?? '-'} />
          <SettingStat label="证书到期" value={service?.cert_not_after ? fmtDateTime(service.cert_not_after) : '未配置'} caption={service?.https_enabled ? 'HTTPS 下次启动生效' : 'HTTP 模式'} />
          <SettingStat label="磁盘预留" value={fmtBytes(min)} caption={`空闲 ${fmtBytes(data?.disk.free_bytes)}`} />
        </div>
      </section>

      <section className="settings-actions-grid">
        <PasswordSettings onDone={refreshOverview} />
        <PortSettings service={service} onDone={refreshOverview} />
        <CertificateSettings service={service} onDone={refreshOverview} />
        <DatabaseSettings data={data} />
        <article className="panel setting-operation">
          <div className="operation-head">
            <div className="operation-icon"><Settings size={18} /></div>
            <div>
              <h2>配置引导</h2>
              <p>重新打开端口、密码和证书配置流程</p>
            </div>
          </div>
          <button className="secondary action-button" type="button" onClick={openWizard}>
            <Settings size={16} />
            打开引导
          </button>
          {message && <p className="error">{message}</p>}
        </article>
      </section>

      {wizardOpen && (
        <SetupWizard
          mode="authenticated"
          status={wizardStatus ?? serviceFromOverview(data)}
          theme={theme}
          onToggleTheme={onToggleTheme}
          onCancel={() => setWizardOpen(false)}
          onComplete={() => {
            setWizardOpen(false);
            setMessage('配置已保存，必要时重启服务后生效');
            void refreshOverview();
          }}
        />
      )}
    </div>
  );
}

function SettingStat({ label, value, caption }: { label: string; value: string; caption: string }) {
  return (
    <div className="setting-stat" data-testid="setting-stat">
      <span>{label}</span>
      <strong>{value}</strong>
      <p>{caption}</p>
    </div>
  );
}

function PasswordSettings({ onDone }: { onDone: () => Promise<void> }) {
  const [currentPassword, setCurrentPassword] = useState('');
  const [nextPassword, setNextPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setMessage('');
    if (nextPassword.length < 8) {
      setMessage('新密码至少 8 位');
      return;
    }
    if (nextPassword !== confirmPassword) {
      setMessage('两次密码不一致');
      return;
    }
    setBusy(true);
    try {
      await changePassword(currentPassword, nextPassword);
      setCurrentPassword('');
      setNextPassword('');
      setConfirmPassword('');
      setMessage('密码已更新');
      await onDone();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'password update failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <article className="panel setting-operation" data-testid="settings-password">
      <div className="operation-head">
        <div className="operation-icon"><LockKeyhole size={18} /></div>
        <div>
          <h2>密码更改</h2>
          <p>仅使用密码作为 Web 凭据</p>
        </div>
      </div>
      <form className="settings-form" onSubmit={submit}>
        <label>当前密码<input value={currentPassword} onChange={(event) => setCurrentPassword(event.target.value)} type="password" autoComplete="current-password" /></label>
        <label>新密码<input value={nextPassword} onChange={(event) => setNextPassword(event.target.value)} type="password" autoComplete="new-password" /></label>
        <label>确认新密码<input value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} type="password" autoComplete="new-password" /></label>
        <button className="primary compact" disabled={busy}><KeyRound size={16} />{busy ? '保存中' : '更新密码'}</button>
      </form>
      {message && <p className={message.includes('已') ? 'notice' : 'error'}>{message}</p>}
    </article>
  );
}

function PortSettings({ service, onDone }: { service?: ServiceStatus; onDone: () => Promise<void> }) {
  const [port, setPort] = useState(String(service?.configured_port || portFromLocation()));
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setPort(String(service?.configured_port || portFromLocation()));
  }, [service?.configured_port]);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setMessage('');
    const parsed = Number(port);
    if (!Number.isInteger(parsed) || parsed < 1 || parsed > 65535) {
      setMessage('端口范围应为 1-65535');
      return;
    }
    setBusy(true);
    try {
      const result = await updateServerConfig(parsed);
      setMessage(result.restart_required ? '端口已保存，重启后生效' : '端口已保存');
      await onDone();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'port update failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <article className="panel setting-operation" data-testid="settings-port">
      <div className="operation-head">
        <div className="operation-icon"><Network size={18} /></div>
        <div>
          <h2>端口配置</h2>
          <p>{service?.current_addr ?? '当前监听端口'}</p>
        </div>
      </div>
      <form className="settings-form inline" onSubmit={submit}>
        <label>访问端口<input value={port} onChange={(event) => setPort(event.target.value)} type="number" min={1} max={65535} inputMode="numeric" /></label>
        <button className="primary compact" disabled={busy}><Save size={16} />{busy ? '保存中' : '保存端口'}</button>
      </form>
      {message && <p className={message.includes('已') ? 'notice' : 'error'}>{message}</p>}
    </article>
  );
}

function CertificateSettings({ service, onDone }: { service?: ServiceStatus; onDone: () => Promise<void> }) {
  const [certificatePEM, setCertificatePEM] = useState('');
  const [privateKeyPEM, setPrivateKeyPEM] = useState('');
  const [certificateName, setCertificateName] = useState('未选择');
  const [keyName, setKeyName] = useState('未选择');
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);

  async function loadPEM(event: React.ChangeEvent<HTMLInputElement>, target: 'cert' | 'key') {
    const file = event.target.files?.[0];
    if (!file) return;
    const text = await file.text();
    if (target === 'cert') {
      setCertificatePEM(text);
      setCertificateName(file.name);
    } else {
      setPrivateKeyPEM(text);
      setKeyName(file.name);
    }
  }

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setMessage('');
    if (!certificatePEM || !privateKeyPEM) {
      setMessage('证书和私钥需要同时上传');
      return;
    }
    setBusy(true);
    try {
      const result = await uploadCertificate(certificatePEM, privateKeyPEM);
      setMessage(result.restart_required ? '证书已保存，重启后启用 HTTPS' : '证书已保存');
      await onDone();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'certificate upload failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <article className="panel setting-operation" data-testid="settings-certificate">
      <div className="operation-head">
        <div className="operation-icon"><FileKey2 size={18} /></div>
        <div>
          <h2>HTTPS 证书</h2>
          <p>到期 {service?.cert_not_after ? fmtDateTime(service.cert_not_after) : '未配置'}</p>
        </div>
      </div>
      <form className="settings-form" onSubmit={submit}>
        <label>证书文件<input type="file" accept=".pem,.crt,.cer" onChange={(event) => loadPEM(event, 'cert')} /><span>{certificateName}</span></label>
        <label>私钥文件<input type="file" accept=".pem,.key" onChange={(event) => loadPEM(event, 'key')} /><span>{keyName}</span></label>
        <button className="primary compact" disabled={busy}><Upload size={16} />{busy ? '上传中' : '上传证书'}</button>
      </form>
      {message && <p className={message.includes('已') ? 'notice' : 'error'}>{message}</p>}
    </article>
  );
}

function DatabaseSettings({ data }: { data?: Overview }) {
  return (
    <article className="panel setting-operation" data-testid="settings-database">
      <div className="operation-head">
        <div className="operation-icon"><Database size={18} /></div>
        <div>
          <h2>数据库下载</h2>
          <p>{fmtHours(data?.retention_hours ?? 0)} · {fmtBytes(data?.disk.free_bytes)} 空闲</p>
        </div>
      </div>
      <a className="secondary action-button" href={databaseDownloadURL()} download>
        <Download size={16} />
        下载数据库
      </a>
    </article>
  );
}

function UtilChart({ items, theme, compact = false }: { items: StoredGPU[]; theme: Theme; compact?: boolean }) {
  const axisColor = theme === 'dark' ? '#9aa8b5' : '#697789';
  const barColor = theme === 'dark' ? '#4db6ac' : '#146c78';
  const option = useMemo(() => ({
    tooltip: {},
    grid: { left: 32, right: 12, top: compact ? 12 : 22, bottom: compact ? 18 : 24 },
    xAxis: { type: 'category', data: items.map((item) => item.gpu.gpu_id), axisLabel: { color: axisColor } },
    yAxis: { type: 'value', max: 100, axisLabel: { color: axisColor }, splitLine: { lineStyle: { color: theme === 'dark' ? '#2c3741' : '#d9e0e7' } } },
    series: [{ type: 'bar', data: items.map((item) => item.gpu.utilization_gpu_percent ?? 0), itemStyle: { color: barColor, borderRadius: [4, 4, 0, 0] } }]
  }), [axisColor, barColor, compact, items, theme]);

  return <EChart option={option} />;
}

function EChart({ option }: { option: echarts.EChartsCoreOption }) {
  const ref = React.useRef<HTMLDivElement>(null);
  React.useEffect(() => {
    if (!ref.current) return;
    const chart = echarts.init(ref.current);
    chart.setOption(option);
    const resize = () => chart.resize();
    window.addEventListener('resize', resize);
    return () => {
      window.removeEventListener('resize', resize);
      chart.dispose();
    };
  }, [option]);
  return <div className="chart" ref={ref} />;
}

createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </React.StrictMode>
);
