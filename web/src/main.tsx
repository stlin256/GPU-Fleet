import React, { useEffect, useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider, useQuery, useQueryClient } from '@tanstack/react-query';
import * as echarts from 'echarts/core';
import { BarChart, LineChart } from 'echarts/charts';
import { GridComponent, TooltipComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import {
  Activity,
  BookOpenText,
  CheckCircle2,
  Clipboard,
  Cpu,
  Database,
  Download,
  FileKey2,
  Gauge,
  Github,
  KeyRound,
  LockKeyhole,
  LogIn,
  LogOut,
  MonitorUp,
  Moon,
  Network,
  PencilLine,
  Plus,
  Power,
  PowerOff,
  RefreshCw,
  Save,
  Server,
  ShieldAlert,
  Settings,
  Sun,
  Trash2,
  Upload
} from 'lucide-react';
import {
  AppLanguage,
  applyUpdate,
  applyInitialSetup,
  applySetup,
  changePassword,
  createDevice,
  databaseDownloadURL,
  deleteDevice,
  Device,
  getGPUSeries,
  getOverview,
  getSetupStatus,
  getStats,
  getUpdateStatus,
  getVersion,
  GPUSeriesPoint,
  GPUStats,
  login,
  logout,
  Overview,
  ReleaseInfo,
  renameDevice,
  reopenSetup,
  rotateDeviceSecret,
  ServiceStatus,
  setDeviceEnabled,
  setAPIErrorFormatter,
  SetupStatus,
  StoredGPU,
  StoredProcess,
  UpdateStatus,
  updateLanguage,
  updateProxy,
  updateServerConfig,
  uploadCertificate
} from './api';
import { I18nContext, installDOMI18n, languages, makeTranslator, useI18n } from './i18n';
import './styles.css';

echarts.use([BarChart, LineChart, GridComponent, TooltipComponent, CanvasRenderer]);

const queryClient = new QueryClient();
type View = 'overview' | 'devices' | 'gpus' | 'settings';
type AuthState = 'checking' | 'setup' | 'authenticated' | 'anonymous';
type Theme = 'light' | 'dark';
type TrendTone = 'good' | 'warn' | 'bad' | 'accent';
type DeviceActionKind = 'enable' | 'disable' | 'rotate' | 'delete';
type PendingUpdateNotice = {
  kind?: 'update' | 'certificate';
  previous_commit?: string;
  target_commit?: string;
  previous_version?: string;
  restart_at?: string;
  started_at: string;
};
type CompletedUpdateNotice = PendingUpdateNotice & {
  product?: string;
  current_commit?: string;
  current_version?: string;
  completed_at: string;
};

const deviceBorderPalette = ['#146c78', '#6750a4', '#b26a00', '#198754', '#c54040', '#2f6fbd', '#8a5a00', '#00806a'];
const repositoryOwner = 'stlin256';
const repositoryName = 'GPU-Fleet';
const repositoryURL = `https://github.com/${repositoryOwner}/${repositoryName}`;
const updatePendingKey = 'gpufleet-update-pending';
const updateNoticeKey = 'gpufleet-update-notice';
const updateStatusCacheKey = 'gpufleet-update-status-cache';
const updateStatusCacheTTL = 60 * 60 * 1000;

type CachedUpdateStatus = {
  status: UpdateStatus;
  cached_at: string;
};

function initialTheme(): Theme {
  const stored = window.localStorage.getItem('gpufleet-theme');
  if (stored === 'light' || stored === 'dark') return stored;
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function initialLanguage(): AppLanguage {
  const stored = window.localStorage.getItem('gpufleet-language');
  if (stored === 'zh-CN' || stored === 'en-US') return stored;
  return navigator.language.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en-US';
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

function fmtMemoryG(used?: number, total?: number) {
  const usedValid = typeof used === 'number' && Number.isFinite(used);
  const totalValid = typeof total === 'number' && Number.isFinite(total) && total > 0;
  const toG = (value: number) => (value / 1024 / 1024 / 1024).toFixed(1);
  if (usedValid && totalValid) return `${toG(used)}/${toG(total)} G`;
  if (usedValid) return `${toG(used)} G`;
  if (totalValid) return `0.0/${toG(total)} G`;
  return '-';
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

function shortHash(value?: string) {
  if (!value) return '-';
  return value.length > 12 ? value.slice(0, 12) : value;
}

function readJSON<T>(key: string): T | undefined {
  try {
    const raw = window.localStorage.getItem(key);
    return raw ? JSON.parse(raw) as T : undefined;
  } catch {
    return undefined;
  }
}

function writeJSON(key: string, value: unknown) {
  window.localStorage.setItem(key, JSON.stringify(value));
}

function storePendingUpdate(notice: PendingUpdateNotice) {
  writeJSON(updatePendingKey, notice);
}

function hasPendingUpdate() {
  return Boolean(readJSON<PendingUpdateNotice>(updatePendingKey));
}

function readCachedUpdateStatus() {
  const cached = readJSON<CachedUpdateStatus>(updateStatusCacheKey);
  if (!cached?.status || !cached.cached_at) return undefined;
  return cached;
}

function storeCachedUpdateStatus(status: UpdateStatus) {
  writeJSON(updateStatusCacheKey, { status, cached_at: new Date().toISOString() } satisfies CachedUpdateStatus);
}

function takeCompletedUpdateNotice() {
  const notice = readJSON<CompletedUpdateNotice>(updateNoticeKey);
  if (notice) window.localStorage.removeItem(updateNoticeKey);
  return notice;
}

async function waitForServerAfterUpdate(pending: PendingUpdateNotice) {
  const deadline = Date.now() + 90_000;
  const minimumWaitUntil = Date.now() + 2_000;
  let sawFailure = false;
  while (Date.now() < deadline) {
    await new Promise((resolve) => window.setTimeout(resolve, 1800));
    try {
      const status = await getUpdateStatus();
      const release = await getVersion().catch(() => undefined);
      const reachedTarget = !pending.target_commit || status.local_commit === pending.target_commit || status.remote_commit === pending.target_commit;
      if (Date.now() >= minimumWaitUntil && (sawFailure || reachedTarget)) {
        window.localStorage.removeItem(updatePendingKey);
        writeJSON(updateNoticeKey, {
          ...pending,
          product: release?.product,
          current_commit: status.local_commit || release?.commit,
          current_version: release?.version,
          completed_at: new Date().toISOString()
        } satisfies CompletedUpdateNotice);
        window.location.reload();
        return;
      }
    } catch {
      sawFailure = true;
    }
  }
}

async function waitForServerAfterRestart(pending: PendingUpdateNotice) {
  const deadline = Date.now() + 90_000;
  const minimumWaitUntil = Date.now() + 2_000;
  let sawFailure = false;
  while (Date.now() < deadline) {
    await new Promise((resolve) => window.setTimeout(resolve, 1800));
    try {
      await getSetupStatus();
      if (Date.now() >= minimumWaitUntil && sawFailure) {
        window.localStorage.removeItem(updatePendingKey);
        writeJSON(updateNoticeKey, {
          ...pending,
          product: 'GPUFleet',
          completed_at: new Date().toISOString()
        } satisfies CompletedUpdateNotice);
        window.location.reload();
        return;
      }
    } catch {
      sawFailure = true;
    }
  }
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
  const [language, setLanguageState] = useState<AppLanguage>(initialLanguage);
  const [updateNotice, setUpdateNotice] = useState<CompletedUpdateNotice | undefined>(() => takeCompletedUpdateNotice());
  const t = useMemo(() => makeTranslator(language), [language]);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
    window.localStorage.setItem('gpufleet-theme', theme);
  }, [theme]);

  useEffect(() => {
    document.documentElement.lang = language === 'zh-CN' ? 'zh-CN' : 'en';
    window.localStorage.setItem('gpufleet-language', language);
    setAPIErrorFormatter((seconds) => t('请求过于频繁，请等待 {duration} 后再试', { duration: retryAfterText(seconds, language) }));
    return installDOMI18n(() => language);
  }, [language, t]);

  function toggleTheme() {
    setTheme((current) => current === 'dark' ? 'light' : 'dark');
  }

  function setLanguage(next: AppLanguage) {
    setLanguageState(next);
  }

  useEffect(() => {
    let cancelled = false;
    const pending = readJSON<PendingUpdateNotice>(updatePendingKey);
    if (pending) {
      if (pending.kind === 'certificate') void waitForServerAfterRestart(pending);
      else void waitForServerAfterUpdate(pending);
    }
    getSetupStatus()
      .then((status) => {
        if (cancelled) return;
        setSetupStatus(status);
        setLanguage(status.service.language || initialLanguage());
        if (status.setup_required) {
          setAuthState('setup');
          return;
        }
        getOverview()
          .then((overview) => {
            if (!cancelled) {
              setLanguage(overview.service.language || status.service.language || initialLanguage());
              setAuthState('authenticated');
            }
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

  return (
    <I18nContext.Provider value={{ language, setLanguage, t }}>
      {authState === 'checking' && <LoadingScreen theme={theme} onToggleTheme={toggleTheme} />}
      {authState === 'setup' && (
        <SetupWizard
          mode="initial"
          status={setupStatus}
          theme={theme}
          onToggleTheme={toggleTheme}
          onComplete={(nextLanguage) => {
            setLanguage(nextLanguage);
            setAuthState('authenticated');
          }}
        />
      )}
      {authState === 'anonymous' && <Login onSuccess={() => setAuthState('authenticated')} theme={theme} onToggleTheme={toggleTheme} />}
      {authState === 'authenticated' && <Dashboard onUnauthorized={() => setAuthState('anonymous')} theme={theme} onToggleTheme={toggleTheme} />}
      <UpdateNoticeDialog notice={updateNotice} onClose={() => setUpdateNotice(undefined)} />
    </I18nContext.Provider>
  );
}

function retryAfterText(seconds: number, language: AppLanguage) {
  const rounded = Math.max(1, Math.ceil(seconds));
  if (language === 'zh-CN') {
    if (rounded >= 3600) return `${Math.ceil(rounded / 3600)} 小时`;
    if (rounded >= 60) return `${Math.ceil(rounded / 60)} 分钟`;
    return `${rounded} 秒`;
  }
  if (rounded >= 3600) return `${Math.ceil(rounded / 3600)} h`;
  if (rounded >= 60) return `${Math.ceil(rounded / 60)} min`;
  return `${rounded} sec`;
}

function LoadingScreen({ theme, onToggleTheme }: { theme: Theme; onToggleTheme: () => void }) {
  return (
    <main className="login-shell">
      <div className="login-panel auth-loading">
        <div className="login-head">
          <Brand />
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
  onComplete: (language: AppLanguage) => void;
  onCancel?: () => void;
}) {
  const { language, setLanguage, t } = useI18n();
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [port, setPort] = useState(String(status?.service.configured_port || portFromLocation()));
  const [selectedLanguage, setSelectedLanguage] = useState<AppLanguage>(status?.service.language || language);
  const [certificatePEM, setCertificatePEM] = useState('');
  const [privateKeyPEM, setPrivateKeyPEM] = useState('');
  const [certificateName, setCertificateName] = useState('');
  const [keyName, setKeyName] = useState('');
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
      setError(t('端口范围应为 1-65535'));
      return;
    }
    if ((requirePassword || password || confirmPassword) && password.length < 8) {
      setError(t('密码至少 8 位'));
      return;
    }
    if (password !== confirmPassword) {
      setError(t('两次密码不一致'));
      return;
    }
    if ((certificatePEM && !privateKeyPEM) || (!certificatePEM && privateKeyPEM)) {
      setError(t('证书和私钥需要同时上传'));
      return;
    }
    setLoading(true);
    try {
      const payload = {
        password: password || undefined,
        port: parsedPort,
        language: selectedLanguage,
        certificate_pem: certificatePEM || undefined,
        private_key_pem: privateKeyPEM || undefined
      };
      const result = mode === 'initial' ? await applyInitialSetup(payload) : await applySetup(payload);
      if (mode === 'initial' && password) {
        await login(password);
      }
      setLanguage(result.service.language || selectedLanguage);
      setMessage(result.restart_required ? t('配置已保存，重启服务后端口或 HTTPS 生效') : t('配置已保存'));
      onComplete(result.service.language || selectedLanguage);
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
          <Brand />
          <ThemeToggle theme={theme} onToggle={onToggleTheme} />
        </div>
        <div className="setup-title">
          <span className="pill good">{service?.current_scheme?.toUpperCase() ?? 'HTTP'}</span>
          <h1>{mode === 'initial' ? t('首次配置') : t('配置引导')}</h1>
          <p>{service ? `${service.current_addr} · ${service.current_scheme.toUpperCase()}` : t('初始化服务访问参数')}</p>
        </div>

        <div className="setup-grid">
          <label>
            {mode === 'initial' ? t('访问密码') : t('新密码')}
            <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="new-password" placeholder={mode === 'initial' ? t('至少 8 位') : t('留空则不变')} />
          </label>
          <label>
            {t('确认密码')}
            <input value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} type="password" autoComplete="new-password" placeholder={mode === 'initial' ? t('再次输入密码') : t('仅修改密码时填写')} />
          </label>
          <label>
            {t('访问端口')}
            <input value={port} onChange={(event) => setPort(event.target.value)} type="number" min={1} max={65535} inputMode="numeric" />
          </label>
          <label>
            {t('界面语言')}
            <select value={selectedLanguage} onChange={(event) => {
              const next = event.target.value as AppLanguage;
              setSelectedLanguage(next);
              setLanguage(next);
            }}>
              {languages.map((item) => <option key={item.code} value={item.code}>{item.nativeLabel}</option>)}
            </select>
          </label>
          <div className="setup-file-row">
            <FilePicker label={t('HTTPS 证书')} accept=".pem,.crt,.cer" fileName={certificateName} onChange={(event) => loadPEM(event, 'cert')} />
            <FilePicker label={t('私钥文件')} accept=".pem,.key" fileName={keyName} onChange={(event) => loadPEM(event, 'key')} />
          </div>
        </div>

        <div className="setup-actions">
          {onCancel && <button className="secondary" type="button" onClick={onCancel}>{t('取消')}</button>}
          <button className="primary compact" disabled={loading}>
            <Save size={17} />
            {loading ? t('保存中') : t('保存配置')}
          </button>
        </div>
        {service?.cert_not_after && <p className="notice">{t('当前证书到期：{date}', { date: fmtDateTime(service.cert_not_after) })}</p>}
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
          <Brand />
          <ThemeToggle theme={theme} onToggle={onToggleTheme} />
        </div>
        <h1>登录面板</h1>
        <p>登录后记住当前设备 30 天</p>
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
  const query = useQueryClient();
  const [view, setView] = useState<View>('overview');
  const overview = useQuery({
    queryKey: ['overview'],
    queryFn: getOverview,
    refetchInterval: 10000,
    retry: 6,
    retryDelay: (attempt) => Math.min(500 * 2 ** attempt, 3000)
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

  useEffect(() => {
    if (hasPendingUpdate()) return undefined;
    let cancelled = false;
    getUpdateStatus()
      .then((status) => {
        if (!cancelled) {
          storeCachedUpdateStatus(status);
          query.setQueryData(['update-status'], status);
        }
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [query]);

  const data = overview.data;
  const statRows = stats.data?.stats ?? [];
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
        <Brand />
        <nav>
          <button className={view === 'overview' ? 'active' : ''} onClick={() => setView('overview')}><Activity size={17} />总览</button>
          <button className={view === 'gpus' ? 'active' : ''} onClick={() => setView('gpus')}><Cpu size={17} />GPU</button>
          <button className={view === 'devices' ? 'active' : ''} onClick={() => setView('devices')}><Server size={17} />设备</button>
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

        <div className="view-shell" key={view} data-view={view}>
          {view === 'overview' && <OverviewPage data={data} statRows={statRows} theme={theme} />}
          {view === 'gpus' && <GPUDetailPage data={data} statRows={statRows} theme={theme} />}
          {view === 'devices' && <DeviceAdminPanel data={data} />}
          {view === 'settings' && <SettingsPanel data={data} theme={theme} onToggleTheme={onToggleTheme} />}
        </div>
      </main>
    </div>
  );
}

function Brand() {
  return (
    <div className="brand" aria-label="GPUFleet">
      <img className="brand-mark" src="/brand/gpufleet-logo.svg" alt="" />
      <span>GPUFleet</span>
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
  const hotCount = gpus.filter((item) => (item.gpu.temperature_celsius ?? 0) >= 80).length;
  const busyCount = gpus.filter((item) => (item.gpu.utilization_gpu_percent ?? 0) >= 80).length;
  const onlineText = data ? `${data.online_device_count}/${data.device_count}` : '-';
  const gpuCountText = data ? String(data.gpu_count) : '-';
  const busyText = data ? String(busyCount) : '-';
  const hotText = data ? String(hotCount) : '-';
  const powerValue = data?.power_draw_watts;

  return (
    <>
      <section className="fleet-command">
        <div className="fleet-command-copy">
          <span className="fleet-eyebrow">Fleet Live</span>
          <h2>多机 GPU 运行态</h2>
          <p>{devices.length > 0 ? `${devices.length} 台设备，${gpus.length} 块 GPU，按最新上报状态汇总。` : '等待客户端上报 GPU 运行信息。'}</p>
        </div>
        <div className="fleet-kpis">
          <FleetKPI label="在线设备" value={onlineText} tone={data && data.online_device_count === data.device_count ? 'good' : 'warn'} />
          <FleetKPI label="GPU 总数" value={gpuCountText} />
          <FleetKPI label="忙碌 GPU" value={busyText} tone={busyCount > 0 ? 'accent' : 'good'} />
          <FleetKPI label="高温 GPU" value={hotText} tone={hotCount > 0 ? 'bad' : 'good'} />
          <FleetKPI label="总显存用量" value={fmtMemoryG(data?.memory_used_bytes, data?.memory_total_bytes)} />
          <FleetKPI label="总功耗" value={typeof powerValue === 'number' ? watts(powerValue) : '-'} tone={(powerValue ?? 0) > 0 ? 'accent' : 'good'} />
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

function GPUDetailPage({ data, statRows, theme }: { data?: Overview; statRows: GPUStats[]; theme: Theme }) {
  return (
    <>
      <section className="stat-grid">
        <Metric icon={<MonitorUp />} label="在线设备" value={`${data?.online_device_count ?? 0} / ${data?.device_count ?? 0}`} />
        <Metric icon={<Cpu />} label="GPU 数量" value={String(data?.gpu_count ?? 0)} />
        <Metric icon={<Gauge />} label="平均利用率" value={pct(data?.average_utilization ?? 0)} />
        <Metric icon={<Database />} label="总显存用量" value={fmtMemoryG(data?.memory_used_bytes, data?.memory_total_bytes)} />
        <Metric icon={<Power />} label="总功耗" value={watts(data?.power_draw_watts ?? 0)} tone={(data?.power_draw_watts ?? 0) > 0 ? 'accent' : 'good'} />
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
  const { language } = useI18n();
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
            <p>{shortGPUName(gpu.name || gpu.gpu_id)} · {gpu.gpu_id} · {timeAgo(item.timestamp, language)}</p>
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
  const timestamps = points.map((point) => point.timestamp);
  const compact = className.includes('detail');
  const memValue = `${pct(mem)} · ${fmtBytes(gpu.memory_used_bytes).replace(' ', '\u00a0')}`;
  const powerValue = watts(gpu.power_draw_watts).replace(' ', '\u00a0');

  return (
    <div className={className}>
      <TrendTile label="GPU 利用率" value={pct(util)} caption={compact && gpu.sm_clock_mhz ? mhz(gpu.sm_clock_mhz).replace(' MHz', '') : gpu.sm_clock_mhz ? mhz(gpu.sm_clock_mhz) : '最近 1 小时'} values={points.map((point) => point.utilization_gpu_percent)} timestamps={timestamps} max={100} tone={metricTone(util, 70, 92)} formatValue={pct} />
      <TrendTile label="显存" value={memValue} caption={compact ? fmtBytes(gpu.memory_total_bytes) : `总量 ${fmtBytes(gpu.memory_total_bytes)}`} values={points.map((point) => point.memory_total_bytes ? (point.memory_used_bytes / point.memory_total_bytes) * 100 : undefined)} timestamps={timestamps} max={100} tone={metricTone(mem, 75, 92)} formatValue={pct} />
      <TrendTile label="温度" value={temp(gpu.temperature_celsius)} caption={tempToneText(gpu.temperature_celsius)} values={points.map((point) => point.temperature_celsius)} timestamps={timestamps} max={100} tone={metricTone(gpu.temperature_celsius, 80, 88)} formatValue={temp} />
      <TrendTile label="功耗" value={powerValue} caption={powerLimit ? (compact ? watts(powerLimit) : `上限 ${watts(powerLimit)}`) : gpu.pstate || '-'} values={points.map((point) => point.power_draw_watts)} timestamps={timestamps} max={powerLimit || maxSeries(points.map((point) => point.power_draw_watts), 200)} tone={metricTone(powerLimit && gpu.power_draw_watts ? (gpu.power_draw_watts / powerLimit) * 100 : undefined, 78, 95)} formatValue={watts} />
    </div>
  );
}

function TrendTile({ label, value, caption, values, timestamps, max, tone, formatValue }: { label: string; value: string; caption: string; values: Array<number | undefined>; timestamps: string[]; max: number; tone: TrendTone; formatValue: (value?: number) => string }) {
  const clean: Array<{ value: number; timestamp?: string }> = [];
  values.forEach((item, index) => {
    if (typeof item === 'number' && Number.isFinite(item)) {
      clean.push({ value: item, timestamp: timestamps[index] });
    }
  });
  return (
    <div className={`trend-tile ${tone}`} data-testid="gpu-trend-tile">
      <div className="trend-head">
        <div>
          <span>{label}</span>
          <strong>{value}</strong>
        </div>
        <p>{caption}</p>
      </div>
      <Sparkline samples={clean} max={max} label={label} formatValue={formatValue} />
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

function Sparkline({ samples, max, label, formatValue }: { samples: Array<{ value: number; timestamp?: string }>; max: number; label: string; formatValue: (value?: number) => string }) {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);
  const width = 180;
  const height = 58;
  const pad = 4;
  const clean = samples.length > 0 ? samples : [{ value: 0 }];
  const cappedMax = Math.max(1, max);
  const pointData = clean.map((sample, index) => {
    const x = clean.length === 1 ? width - pad : pad + (index / (clean.length - 1)) * (width - pad * 2);
    const y = height - pad - (Math.max(0, Math.min(cappedMax, sample.value)) / cappedMax) * (height - pad * 2);
    return { ...sample, x, y };
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
          <small>{active.timestamp ? fmtDateTime(active.timestamp) : '-'}</small>
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

function timeAgo(value: string, language: AppLanguage) {
  const delta = Date.now() - new Date(value).getTime();
  if (!Number.isFinite(delta) || delta < 0) return language === 'en-US' ? 'just now' : '刚刚';
  const seconds = Math.floor(delta / 1000);
  if (seconds < 60) return language === 'en-US' ? `${seconds}s ago` : `${seconds}s 前`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return language === 'en-US' ? `${minutes}m ago` : `${minutes}m 前`;
  const hours = Math.floor(minutes / 60);
  return language === 'en-US' ? `${hours}h ago` : `${hours}h 前`;
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
  const [confirm, setConfirm] = useState<{ kind: DeviceActionKind; device: Device }>();
  const [editingDevice, setEditingDevice] = useState<{ id: string; alias: string }>();

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

  async function runConfirmedAction() {
    if (!confirm) return;
    const { kind, device } = confirm;
    setConfirm(undefined);
    setBusy(`${kind}-${device.id}`);
    setMessage('');
    try {
      if (kind === 'enable' || kind === 'disable') {
        await setDeviceEnabled(device.id, kind === 'enable');
        setMessage(kind === 'enable' ? '设备已启用' : '设备已禁用');
      }
      if (kind === 'rotate') {
        const result = await rotateDeviceSecret(device.id);
        setSecret({ deviceId: device.id, value: result.secret, title: '已轮换密钥' });
        setMessage('设备密钥已轮换');
      }
      if (kind === 'delete') {
        await deleteDevice(device.id);
        setSecret(undefined);
        setMessage('设备已删除');
      }
      await refresh();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'device action failed');
    } finally {
      setBusy('');
    }
  }

  async function saveDeviceAlias(deviceId: string) {
    const nextAlias = editingDevice?.alias.trim() ?? '';
    setBusy(`rename-${deviceId}`);
    setMessage('');
    try {
      const result = await renameDevice(deviceId, nextAlias);
      setMessage(`设备名称已更新为 ${result.device.alias || result.device.id}`);
      setEditingDevice(undefined);
      await refresh();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'device rename failed');
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
              <div className="device-name-cell">
                {editingDevice?.id === device.id ? (
                  <form className="device-rename-form" onSubmit={(event) => {
                    event.preventDefault();
                    void saveDeviceAlias(device.id);
                  }}>
                    <input
                      value={editingDevice.alias}
                      onChange={(event) => setEditingDevice({ id: device.id, alias: event.target.value })}
                      aria-label="设备名称"
                      autoFocus
                    />
                    <button className="secondary" type="submit" disabled={busy === `rename-${device.id}`}>
                      <Save size={16} />
                      保存
                    </button>
                    <button className="secondary" type="button" onClick={() => setEditingDevice(undefined)} disabled={busy === `rename-${device.id}`}>取消</button>
                  </form>
                ) : (
                  <>
                    <strong>{device.alias || device.id}</strong>
                    <p>{device.id} · {device.hostname || '-'} · {device.agent_version || '-'}</p>
                  </>
                )}
              </div>
              <span className={`pill ${device.enabled ? (device.status ?? 'offline') : 'disabled'}`}>{device.enabled ? (device.status ?? 'offline') : 'disabled'}</span>
              <div className="row-actions">
                <button className="secondary" onClick={() => setEditingDevice({ id: device.id, alias: device.alias || device.id })} disabled={Boolean(editingDevice) || busy.endsWith(device.id)} title="修改设备名称">
                  <PencilLine size={16} />
                  改名
                </button>
                <button className="secondary" onClick={() => setConfirm({ kind: device.enabled ? 'disable' : 'enable', device })} disabled={Boolean(editingDevice) || busy.endsWith(device.id)} title={device.enabled ? '禁用设备' : '启用设备'}>
                  {device.enabled ? <PowerOff size={16} /> : <Power size={16} />}
                  {device.enabled ? '禁用' : '启用'}
                </button>
                <button className="secondary" onClick={() => setConfirm({ kind: 'rotate', device })} disabled={Boolean(editingDevice) || busy === `rotate-${device.id}`} title="轮换密钥">
                  <KeyRound size={16} />
                  轮换
                </button>
                <button className="secondary danger-action" onClick={() => setConfirm({ kind: 'delete', device })} disabled={Boolean(editingDevice) || busy === `delete-${device.id}`} title="删除设备">
                  <Trash2 size={16} />
                  删除
                </button>
              </div>
            </div>
          ))}
          {(data?.devices ?? []).length === 0 && <p className="empty">暂无设备</p>}
        </div>
      </section>
      <DeviceActionConfirm
        confirm={confirm}
        busy={Boolean(busy)}
        onCancel={() => setConfirm(undefined)}
        onConfirm={runConfirmedAction}
      />
    </div>
  );
}

function DeviceActionConfirm({
  confirm,
  busy,
  onCancel,
  onConfirm
}: {
  confirm?: { kind: DeviceActionKind; device: Device };
  busy: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  useEffect(() => {
    if (!confirm) return undefined;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onCancel();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [confirm, onCancel]);

  if (!confirm) return null;
  const copy = deviceActionCopy(confirm.kind, confirm.device);

  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onCancel();
    }}>
      <section className={`confirm-dialog ${copy.tone}`} role="dialog" aria-modal="true" aria-labelledby="device-confirm-title" data-testid="device-confirm-dialog">
        <div className="confirm-icon"><ShieldAlert size={22} /></div>
        <div className="confirm-copy">
          <span>{confirm.device.id}</span>
          <h2 id="device-confirm-title">{copy.title}</h2>
          <p>{copy.body}</p>
        </div>
        <div className="confirm-target">
          <span>目标设备</span>
          <strong>{confirm.device.alias || confirm.device.id}</strong>
        </div>
        <div className="confirm-actions">
          <button className="secondary" type="button" onClick={onCancel} disabled={busy}>取消</button>
          <button className={`primary compact ${copy.tone === 'danger' ? 'danger-primary' : ''}`} type="button" onClick={onConfirm} disabled={busy}>
            {copy.icon}
            {busy ? '处理中' : copy.confirmLabel}
          </button>
        </div>
      </section>
    </div>
  );
}

function UpdateNoticeDialog({ notice, onClose }: { notice?: CompletedUpdateNotice; onClose: () => void }) {
  const { t } = useI18n();

  useEffect(() => {
    if (!notice) return undefined;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [notice, onClose]);

  if (!notice) return null;
  const isCertificate = notice.kind === 'certificate';
  const from = shortHash(notice.previous_commit);
  const to = shortHash(notice.current_commit || notice.target_commit);
  const versionText = notice.current_version ? `v${notice.current_version}` : '-';
  const title = isCertificate ? t('HTTPS 证书已启用') : t('版本已更新');
  const body = isCertificate ? t('HTTPS 证书已保存，服务端已自动重启并刷新页面。') : t('服务端已自动重启并刷新页面。');

  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <section className="confirm-dialog update-notice-dialog" role="dialog" aria-modal="true" aria-labelledby="update-notice-title" data-testid="update-notice-dialog">
        <div className="confirm-icon"><CheckCircle2 size={22} /></div>
        <div className="confirm-copy">
          <span>{notice.product || 'GPUFleet'}</span>
          <h2 id="update-notice-title">{title}</h2>
          <p>{body}</p>
        </div>
        {!isCertificate && <div className="confirm-target update-notice-grid">
          <div>
            <span>{t('版本')}</span>
            <strong>{versionText}</strong>
          </div>
          <div>
            <span>{t('提交')}</span>
            <strong title={notice.current_commit || notice.target_commit}>{from !== '-' && to !== '-' ? `${from} -> ${to}` : to}</strong>
          </div>
        </div>}
        <div className="confirm-actions">
          <button className="primary compact" type="button" onClick={onClose}>
            <CheckCircle2 size={16} />
            {t('知道了')}
          </button>
        </div>
      </section>
    </div>
  );
}

function deviceActionCopy(kind: DeviceActionKind, device: Device) {
  const name = device.alias || device.id;
  if (kind === 'enable') {
    return {
      title: '启用设备',
      body: `允许 ${name} 使用现有密钥继续上报 GPU 指标。`,
      confirmLabel: '确认启用',
      tone: 'normal',
      icon: <Power size={16} />
    };
  }
  if (kind === 'disable') {
    return {
      title: '禁用设备',
      body: `禁用后 ${name} 的上报请求会被服务端拒绝，客户端本机配置不会被修改。`,
      confirmLabel: '确认禁用',
      tone: 'warning',
      icon: <PowerOff size={16} />
    };
  }
  if (kind === 'rotate') {
    return {
      title: '轮换密钥',
      body: `旧密钥会立即失效，需要在 ${name} 所在机器手动更新新密钥后才能继续上报。`,
      confirmLabel: '确认轮换',
      tone: 'warning',
      icon: <KeyRound size={16} />
    };
  }
  return {
    title: '删除设备',
    body: `删除后 ${name} 将从设备列表和最新 GPU 快照中移除，原 Agent 密钥会失效。`,
    confirmLabel: '确认删除',
    tone: 'danger',
    icon: <Trash2 size={16} />
  };
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
  const { setLanguage, t } = useI18n();
  const service = data?.service;
  const min = data?.min_free_space_bytes ?? data?.disk.min_free_bytes ?? 0;
  const [wizardOpen, setWizardOpen] = useState(false);
  const [wizardStatus, setWizardStatus] = useState<SetupStatus>();
  const [message, setMessage] = useState('');
  const release = useQuery({
    queryKey: ['version'],
    queryFn: getVersion,
    staleTime: 5 * 60 * 1000
  });

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
  const certCaption = service?.https_enabled
    ? service.current_scheme === 'https' ? 'HTTPS 已启用' : 'HTTPS 下次启动生效'
    : 'HTTP 模式';

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
          <SettingStat label="证书到期" value={service?.cert_not_after ? fmtDateTime(service.cert_not_after) : '未配置'} caption={certCaption} />
          <SettingStat label="磁盘预留" value={fmtBytes(min)} caption={`空闲 ${fmtBytes(data?.disk.free_bytes)}`} />
        </div>
      </section>

      <section className="settings-workbench">
        <div className="settings-column">
          <div className="settings-section-head">
            <div>
              <h2>访问与安全</h2>
              <p>凭据、端口、语言和 HTTPS 证书</p>
            </div>
          </div>
          <PasswordSettings onDone={refreshOverview} />
          <PortSettings service={service} onDone={refreshOverview} />
          <LanguageSettings service={service} onDone={refreshOverview} />
          <CertificateSettings service={service} onDone={refreshOverview} />
          <article className="panel setting-operation">
            <div className="operation-head">
              <div className="operation-icon"><Settings size={18} /></div>
              <div>
                <h2>配置引导</h2>
                <p>重新打开端口、密码、语言和证书配置流程</p>
              </div>
            </div>
            <button className="secondary action-button" type="button" onClick={openWizard}>
              <Settings size={16} />
              打开引导
            </button>
            {message && <p className="error">{message}</p>}
          </article>
        </div>

        <div className="settings-column settings-column-operations">
          <div className="settings-section-head">
            <div>
              <h2>维护与发布</h2>
              <p>数据库、在线更新和版本信息</p>
            </div>
          </div>
          <DatabaseSettings data={data} />
          <UpdateSettings service={service} onDone={refreshOverview} />
          <ProjectInfoSettings release={release.data} loading={release.isLoading} error={release.error instanceof Error ? release.error.message : ''} />
        </div>
      </section>

      {wizardOpen && (
        <SetupWizard
          mode="authenticated"
          status={wizardStatus ?? serviceFromOverview(data)}
          theme={theme}
          onToggleTheme={onToggleTheme}
          onCancel={() => setWizardOpen(false)}
          onComplete={(nextLanguage) => {
            setLanguage(nextLanguage);
            setWizardOpen(false);
            setMessage(t('配置已保存，必要时重启服务后生效'));
            void refreshOverview();
          }}
        />
      )}
    </div>
  );
}

function UpdateSettings({ service, onDone }: { service?: ServiceStatus; onDone: () => Promise<void> }) {
  const query = useQueryClient();
  const { t } = useI18n();
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);
  const [waitingForRestart, setWaitingForRestart] = useState(() => hasPendingUpdate());
  const [proxyURL, setProxyURL] = useState(service?.update_proxy || '');
  const [proxyMessage, setProxyMessage] = useState('');
  const [savingProxy, setSavingProxy] = useState(false);
  const [progressStep, setProgressStep] = useState(0);
  const cachedUpdate = readCachedUpdateStatus();
  const cachedUpdateAge = cachedUpdate ? Date.now() - new Date(cachedUpdate.cached_at).getTime() : Number.POSITIVE_INFINITY;
  const release = useQuery({
    queryKey: ['version'],
    queryFn: getVersion,
    staleTime: 5 * 60 * 1000
  });
  const update = useQuery({
    queryKey: ['update-status'],
    queryFn: async () => {
      const next = await getUpdateStatus();
      storeCachedUpdateStatus(next);
      return next;
    },
    initialData: cachedUpdate?.status,
    initialDataUpdatedAt: cachedUpdate ? new Date(cachedUpdate.cached_at).getTime() : undefined,
    enabled: !waitingForRestart,
    staleTime: updateStatusCacheTTL,
    refetchInterval: waitingForRestart ? false : updateStatusCacheTTL,
    refetchOnWindowFocus: false,
    refetchOnMount: !waitingForRestart && cachedUpdateAge >= updateStatusCacheTTL,
    retry: false
  });
  const status = update.data;
  const state = waitingForRestart
    ? { label: t('重启中'), tone: 'warn' as const, message: t('服务端正在自动重启，恢复后页面会自动刷新。') }
    : updateState(status, update.isLoading, update.error instanceof Error ? update.error.message : '');
  const canApply = Boolean(status?.supported && status.upstream && (status.available || status.binary_outdated) && !status.dirty && status.ahead === 0 && !busy);

  useEffect(() => {
    setProxyURL(service?.update_proxy || '');
  }, [service?.update_proxy]);

  async function check() {
    setMessage('');
    setProgressStep(0);
    setWaitingForRestart(false);
    const result = await update.refetch();
    if (result.data) storeCachedUpdateStatus(result.data);
  }

  async function saveProxy(event: React.FormEvent) {
    event.preventDefault();
    setProxyMessage('');
    setSavingProxy(true);
    try {
      await updateProxy(proxyURL.trim());
      setProxyMessage(proxyURL.trim() ? '更新代理已保存' : '更新代理已清空');
      await onDone();
      await update.refetch();
    } catch (err) {
      const message = err instanceof Error ? err.message : 'proxy update failed';
      setProxyMessage(message.toLowerCase().includes('not found') ? '当前服务端未包含更新代理接口，请先完成服务端更新并重启' : message);
    } finally {
      setSavingProxy(false);
    }
  }

  async function pull() {
    setMessage('');
    setBusy(true);
    setProgressStep(1);
    const timer = window.setTimeout(() => setProgressStep(2), 1200);
    try {
      const result = await applyUpdate();
      window.clearTimeout(timer);
      setProgressStep(3);
      if (result.restarting) {
        setProgressStep(4);
        const pending = {
          previous_commit: status?.local_commit,
          target_commit: result.status.local_commit || status?.remote_commit,
          previous_version: release.data?.version,
          restart_at: result.restart_at,
          started_at: new Date().toISOString()
        };
        storePendingUpdate(pending);
        setWaitingForRestart(true);
        setMessage(`更新已构建完成，服务端正在自动重启${result.restart_at ? `，预计 ${fmtDateTime(result.restart_at)} 前后恢复` : ''}。恢复后页面会自动刷新。`);
        setProgressStep(5);
        void waitForServerAfterUpdate(pending);
      } else {
        if (result.restart_required) {
          setProgressStep(5);
          const notice = {
            previous_commit: status?.local_commit,
            target_commit: result.status.local_commit || status?.remote_commit,
            previous_version: release.data?.version,
            current_commit: result.status.local_commit,
            current_version: release.data?.version,
            started_at: new Date().toISOString(),
            completed_at: new Date().toISOString()
          } satisfies CompletedUpdateNotice;
          writeJSON(updateNoticeKey, notice);
          window.location.reload();
          return;
        }
        setProgressStep(6);
        setMessage('当前已经是最新版本');
      }
      if (!result.restarting) await query.invalidateQueries({ queryKey: ['update-status'] });
      await query.invalidateQueries({ queryKey: ['version'] });
    } catch (err) {
      window.clearTimeout(timer);
      setProgressStep(0);
      setMessage(err instanceof Error ? err.message : 'update failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <article className={`panel setting-operation update-card ${state.tone}`} data-testid="settings-update">
      <div className="operation-head">
        <div className="operation-icon"><Download size={18} /></div>
        <div>
          <h2>在线更新</h2>
          <p>{status?.branch ? `${status.branch} · ${status.upstream || '未绑定上游'}` : '检查 Git 上游版本'}</p>
        </div>
        <span className={`pill ${state.tone}`}>{state.label}</span>
      </div>

      <div className="update-compare">
        <div>
          <span>当前提交</span>
          <strong title={status?.local_commit}>{shortHash(status?.local_commit)}</strong>
        </div>
        <div>
          <span>远端提交</span>
          <strong title={status?.remote_commit}>{shortHash(status?.remote_commit)}</strong>
        </div>
        <div>
          <span>落后</span>
          <strong>{status?.behind ?? 0}</strong>
        </div>
        <div>
          <span>超前</span>
          <strong>{status?.ahead ?? 0}</strong>
        </div>
      </div>

      <div className="update-meta">
        <div>
          <span>运行版本</span>
          <strong>{status?.running_version ? `v${status.running_version}` : '-'}</strong>
        </div>
        <div>
          <span>仓库版本</span>
          <strong>{status?.repo_version ? `v${status.repo_version}` : '-'}</strong>
        </div>
        <div>
          <span>运行提交</span>
          <strong title={status?.running_commit}>{shortHash(status?.running_commit)}</strong>
        </div>
        <div>
          <span>远端</span>
          <strong title={status?.remote}>{status?.remote || '-'}</strong>
        </div>
        <div>
          <span>检查时间</span>
          <strong>{fmtDateTime(status?.checked_at)}</strong>
        </div>
      </div>

      <form className="settings-form inline update-proxy-form" onSubmit={saveProxy}>
        <label>
          更新代理
          <input value={proxyURL} onChange={(event) => setProxyURL(event.target.value)} placeholder="http://127.0.0.1:7890" />
        </label>
        <button className="secondary" type="submit" disabled={savingProxy || busy}>
          <Network size={16} />
          {savingProxy ? '保存中' : '保存代理'}
        </button>
      </form>
      {proxyMessage && <p className={proxyMessage.includes('已') ? 'notice update-note' : 'error update-note'}>{proxyMessage}</p>}

      <div className="settings-button-row">
        <button className="secondary" type="button" onClick={check} disabled={update.isFetching || busy}>
          <RefreshCw size={16} />
          {update.isFetching ? '检查中' : '检查更新'}
        </button>
        <button className="primary compact" type="button" onClick={pull} disabled={!canApply}>
          <Download size={16} />
          {busy ? '更新中' : status?.binary_outdated && !status.available ? '重建并重启' : '拉取并重启'}
        </button>
      </div>
      {progressStep > 0 && <UpdateProgress step={progressStep} />}
      {(message || state.message) && <p className={message.includes('已') || message.includes('正在自动重启') || state.tone === 'good' ? 'notice update-note' : 'error update-note'}>{message || state.message}</p>}
    </article>
  );
}

function UpdateProgress({ step }: { step: number }) {
  const stages = [
    '已发送更新请求',
    '依赖预检、构建远端提交并执行 fast-forward 拉取',
    '更新已应用，准备自动重启',
    '服务端正在自动重启',
    '等待服务端恢复，恢复后自动刷新'
  ];
  return (
    <div className="update-progress" data-testid="update-progress">
      {stages.map((label, index) => (
        <div className={index + 1 < step ? 'done' : index + 1 === step ? 'active' : ''} key={label}>
          <span>{index + 1}</span>
          <p>{label}</p>
        </div>
      ))}
    </div>
  );
}

function updateState(status?: UpdateStatus, loading = false, error = '') {
  if (loading) return { label: '检查中', tone: 'warn', message: '正在读取 Git 状态' };
  if (error) return { label: '失败', tone: 'bad', message: error };
  if (!status) return { label: '未知', tone: 'warn', message: '尚未读取更新状态' };
  if (!status.supported) return { label: '不可用', tone: 'bad', message: status.message || '服务端未运行在 Git 工作区' };
  if (status.dirty) return { label: '已阻止', tone: 'bad', message: '服务端工作区存在未提交改动，已阻止自动拉取' };
  if (!status.upstream) return { label: '未绑定', tone: 'warn', message: status.message || '当前分支没有 Git upstream' };
  if (status.ahead > 0 && status.behind > 0) return { label: '分叉', tone: 'bad', message: '本地和上游存在分叉，不能自动 fast-forward' };
  if (status.ahead > 0) return { label: '本地超前', tone: 'warn', message: '本地提交超前上游，面板不会执行拉取' };
  if (status.available) return { label: '有新版本', tone: 'good', message: `${status.behind} 个提交可拉取、构建并自动重启` };
  if (status.binary_outdated) return { label: '需重建', tone: 'warn', message: '运行中的服务端二进制与当前仓库不一致，可重建并自动重启' };
  return { label: '最新', tone: 'good', message: status.message || '已经是最新版本' };
}

function ProjectInfoSettings({ release, loading, error }: { release?: ReleaseInfo; loading: boolean; error: string }) {
  const { language } = useI18n();
  const latest = release?.changelog?.[0];
  const versionText = release?.version ? `v${release.version}` : loading ? '加载中' : '-';
  const commitText = release?.commit && release.commit !== 'dev' ? release.commit : 'dev';

  return (
    <article className="panel setting-operation project-card release-card" data-testid="settings-project">
      <div className="operation-head">
        <div className="operation-icon project-logo">
          <img src="/brand/gpufleet-logo.svg" alt="" />
        </div>
        <div>
          <h2>版本与变更</h2>
          <p>{release ? `${release.product} ${versionText}` : 'GPUFleet 发布信息'}</p>
        </div>
      </div>
      <div className="project-meta">
        <div>
          <span>作者</span>
          <strong>{release?.author ?? repositoryOwner}</strong>
        </div>
        <div>
          <span>版本</span>
          <strong>{versionText}</strong>
        </div>
        <div>
          <span>提交</span>
          <strong>{commitText}</strong>
        </div>
        <div>
          <span>构建时间</span>
          <strong>{release?.build_time ? fmtDateTime(release.build_time) : '-'}</strong>
        </div>
        <div className="project-url">
          <span>仓库地址</span>
          <a href={release?.repository ?? repositoryURL} target="_blank" rel="noreferrer">{release?.repository ?? repositoryURL}</a>
        </div>
      </div>
      <div className="changelog-panel" data-testid="settings-changelog">
        <div className="changelog-head">
          <BookOpenText size={16} />
          <span>最近变更</span>
        </div>
        {latest ? <ChangelogEntryView entry={latest} language={language} /> : <p>{error || '正在读取版本信息'}</p>}
      </div>
      <a className="secondary action-button" href={release?.repository ?? repositoryURL} target="_blank" rel="noreferrer">
        <Github size={16} />
        打开 GitHub
      </a>
    </article>
  );
}

function ChangelogEntryView({ entry, language }: { entry: NonNullable<ReleaseInfo['changelog']>[number]; language: AppLanguage }) {
  const title = language === 'en-US' ? entry.title_en || entry.title : entry.title;
  const added = language === 'en-US' ? entry.added_en || entry.added : entry.added;
  const changed = language === 'en-US' ? entry.changed_en || entry.changed : entry.changed;
  const security = language === 'en-US' ? entry.security_en || entry.security : entry.security;
  const fixed = language === 'en-US' ? entry.fixed_en || entry.fixed : entry.fixed;
  return (
    <div className="changelog-entry">
      <div>
        <strong>v{entry.version}</strong>
        <span>{entry.date}</span>
      </div>
      <p>{title}</p>
      <ChangelogList label="新增" items={added} />
      <ChangelogList label="变更" items={changed} />
      <ChangelogList label="安全" items={security} />
      <ChangelogList label="修复" items={fixed} />
    </div>
  );
}

function ChangelogList({ label, items }: { label: string; items?: string[] }) {
  if (!items?.length) return null;
  return (
    <div className="changelog-group">
      <span>{label}</span>
      <ul>
        {items.slice(0, 4).map((item) => <li key={item}>{item}</li>)}
      </ul>
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

function LanguageSettings({ service, onDone }: { service?: ServiceStatus; onDone: () => Promise<void> }) {
  const { language, setLanguage, t } = useI18n();
  const [selectedLanguage, setSelectedLanguage] = useState<AppLanguage>(service?.language || language);
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setSelectedLanguage(service?.language || language);
  }, [language, service?.language]);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setMessage('');
    setBusy(true);
    try {
      const result = await updateLanguage(selectedLanguage);
      setLanguage(result.service.language || selectedLanguage);
      setMessage(t('语言已保存'));
      await onDone();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'language update failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <article className="panel setting-operation" data-testid="settings-language">
      <div className="operation-head">
        <div className="operation-icon"><Settings size={18} /></div>
        <div>
          <h2>{t('语言设置')}</h2>
          <p>{t('控制首次配置、面板和后续设置页语言')}</p>
        </div>
      </div>
      <form className="settings-form inline" onSubmit={submit}>
        <label>
          {t('界面语言')}
          <select value={selectedLanguage} onChange={(event) => setSelectedLanguage(event.target.value as AppLanguage)}>
            {languages.map((item) => <option key={item.code} value={item.code}>{item.nativeLabel}</option>)}
          </select>
        </label>
        <button className="primary compact" disabled={busy}><Save size={16} />{busy ? t('保存中') : t('保存语言')}</button>
      </form>
      {message && <p className={message.includes('已') || message.includes('saved') ? 'notice' : 'error'}>{message}</p>}
    </article>
  );
}

function CertificateSettings({ service, onDone }: { service?: ServiceStatus; onDone: () => Promise<void> }) {
  const { t } = useI18n();
  const [certificatePEM, setCertificatePEM] = useState('');
  const [privateKeyPEM, setPrivateKeyPEM] = useState('');
  const [certificateName, setCertificateName] = useState('');
  const [keyName, setKeyName] = useState('');
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
      setMessage(t('证书和私钥需要同时上传'));
      return;
    }
    setBusy(true);
    try {
      const result = await uploadCertificate(certificatePEM, privateKeyPEM);
      if (result.restarting) {
        const pending = {
          kind: 'certificate',
          restart_at: result.restart_at,
          started_at: new Date().toISOString()
        } satisfies PendingUpdateNotice;
        storePendingUpdate(pending);
        setMessage(t('证书已保存，服务端正在自动重启。恢复后页面会自动刷新。'));
        void waitForServerAfterRestart(pending);
        return;
      }
      setMessage(result.restart_required ? t('证书已保存，重启后启用 HTTPS') : t('证书已保存'));
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
          <h2>{t('HTTPS 证书')}</h2>
          <p>{t('到期 {date}', { date: service?.cert_not_after ? fmtDateTime(service.cert_not_after) : t('未配置') })}</p>
        </div>
      </div>
      <form className="settings-form" onSubmit={submit}>
        <FilePicker label={t('证书文件')} accept=".pem,.crt,.cer" fileName={certificateName} onChange={(event) => loadPEM(event, 'cert')} />
        <FilePicker label={t('私钥文件')} accept=".pem,.key" fileName={keyName} onChange={(event) => loadPEM(event, 'key')} />
        <button className="primary compact" disabled={busy}><Upload size={16} />{busy ? t('上传中') : t('上传证书')}</button>
      </form>
      {message && <p className={message.includes('已') || message.includes('saved') ? 'notice' : 'error'}>{message}</p>}
    </article>
  );
}

function FilePicker({ label, accept, fileName, onChange }: { label: string; accept: string; fileName: string; onChange: (event: React.ChangeEvent<HTMLInputElement>) => void }) {
  const { t } = useI18n();
  const id = React.useId();
  return (
    <label className="file-picker" htmlFor={id}>
      <span>{label}</span>
      <input id={id} type="file" accept={accept} onChange={onChange} />
      <span className="file-picker-control">
        <span className="secondary file-picker-button">{t('选择文件')}</span>
        <span className="file-picker-name">{fileName || t('未选择文件')}</span>
      </span>
    </label>
  );
}

function DatabaseSettings({ data }: { data?: Overview }) {
  return (
    <article className="panel setting-operation" data-testid="settings-database">
      <div className="operation-head">
        <div className="operation-icon"><Database size={18} /></div>
        <div>
          <h2>数据库下载</h2>
          <p>数据库大小 {fmtBytes(data?.database_size_bytes ?? 0)} · {fmtHours(data?.retention_hours ?? 0)} · {fmtBytes(data?.disk.free_bytes)} 空闲</p>
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
