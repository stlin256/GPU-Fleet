import React, { useEffect, useMemo, useState } from 'react';
import { createPortal } from 'react-dom';
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
  CircleHelp,
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
  getGuestGPUSeries,
  getGuestOverview,
  getGuestStatus,
  getGuestVisits,
  getOverview,
  getSetupStatus,
  getStats,
  getUpdateNotice,
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
  restartService,
  rotateDeviceSecret,
  ServiceStatus,
  setDeviceEnabled,
  setAPIErrorFormatter,
  SetupStatus,
  StoredGPU,
  StoredProcess,
  UpdateStatus,
  UpdateNotice,
  updateLanguage,
  updateGuest,
  updateProxy,
  updateServerConfig,
  uploadCertificate,
  GuestVisit
} from './api';
import { I18nContext, installDOMI18n, languages, makeTranslator, useI18n } from './i18n';
import './styles.css';

echarts.use([BarChart, LineChart, GridComponent, TooltipComponent, CanvasRenderer]);

const queryClient = new QueryClient();
type View = 'overview' | 'devices' | 'gpus' | 'settings';
type AuthState = 'checking' | 'setup' | 'authenticated' | 'anonymous' | 'guest';
type Theme = 'light' | 'dark';
type TrendTone = 'good' | 'warn' | 'bad' | 'accent';
type DeviceActionKind = 'enable' | 'disable' | 'rotate' | 'delete';
type PendingUpdateNotice = {
  kind?: 'auto_update' | 'update' | 'certificate' | 'restart';
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
  updated_at?: string;
  summary?: string[];
  summary_en?: string[];
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
const autoUpdateStatusTTL = 30 * 60 * 1000;

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
  return browserLanguage();
}

function browserLanguage(): AppLanguage {
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

function storedDaysValue(days?: number, fallbackHours?: number) {
  const fallbackDays = typeof fallbackHours === 'number' && Number.isFinite(fallbackHours) && fallbackHours > 0 ? Math.ceil(fallbackHours / 24) : 0;
  const value = typeof days === 'number' && Number.isFinite(days) ? days : fallbackDays;
  return Math.max(0, value);
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

function clearCachedUpdateStatus() {
  window.localStorage.removeItem(updateStatusCacheKey);
}

function takeCompletedUpdateNotice() {
  const notice = readJSON<CompletedUpdateNotice>(updateNoticeKey);
  if (notice) {
    window.localStorage.removeItem(updateNoticeKey);
    clearCachedUpdateStatus();
  }
  return notice;
}

function completedNoticeFromServer(notice?: UpdateNotice): CompletedUpdateNotice | undefined {
  if (!notice) return undefined;
  return {
    kind: notice.kind,
    product: notice.product,
    previous_commit: notice.previous_commit,
    target_commit: notice.target_commit,
    current_commit: notice.current_commit,
    previous_version: notice.previous_version,
    current_version: notice.current_version,
    started_at: notice.started_at,
    completed_at: notice.completed_at || notice.updated_at || new Date().toISOString(),
    updated_at: notice.updated_at,
    summary: notice.summary,
    summary_en: notice.summary_en
  };
}

async function waitForServerAfterUpdate(pending: PendingUpdateNotice) {
  const deadline = Date.now() + 90_000;
  const minimumWaitUntil = Date.now() + 2_000;
  let sawFailure = false;
  while (Date.now() < deadline) {
    await new Promise((resolve) => window.setTimeout(resolve, 1800));
    try {
      const status = await getUpdateStatus(true);
      const release = await getVersion().catch(() => undefined);
      const reachedTarget = !pending.target_commit || status.local_commit === pending.target_commit || status.remote_commit === pending.target_commit;
      if (Date.now() >= minimumWaitUntil && (sawFailure || reachedTarget)) {
        window.localStorage.removeItem(updatePendingKey);
        storeCachedUpdateStatus(status);
        const serverNotice = await getUpdateNotice()
          .then((result) => completedNoticeFromServer(result.notice))
          .catch(() => undefined);
        writeJSON(updateNoticeKey, serverNotice ?? {
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
  const [guestEnabled, setGuestEnabled] = useState(false);
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

  async function fetchServerUpdateNotice() {
    try {
      const result = await getUpdateNotice();
      const notice = completedNoticeFromServer(result.notice);
      if (notice) {
        clearCachedUpdateStatus();
        queryClient.removeQueries({ queryKey: ['update-status'] });
        setUpdateNotice(notice);
      }
    } catch {
      // Older servers do not expose the notice endpoint; the local restart notice still works.
    }
  }

  useEffect(() => {
    let cancelled = false;
    const pending = readJSON<PendingUpdateNotice>(updatePendingKey);
    if (pending) {
      if (pending.kind === 'certificate' || pending.kind === 'restart') void waitForServerAfterRestart(pending);
      else void waitForServerAfterUpdate(pending);
    }
    if (window.location.pathname === '/guest') {
      getGuestStatus()
        .then((status) => {
          if (cancelled) return;
          setGuestEnabled(status.guest_enabled);
          setLanguage(browserLanguage());
          setAuthState(status.guest_enabled ? 'guest' : 'anonymous');
        })
        .catch(() => {
          if (!cancelled) setAuthState('anonymous');
        });
      return () => {
        cancelled = true;
      };
    }
    getSetupStatus()
      .then((status) => {
        if (cancelled) return;
        setSetupStatus(status);
        setGuestEnabled(status.service.guest_enabled);
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
              void fetchServerUpdateNotice();
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
      {authState === 'anonymous' && <Login onSuccess={() => {
        setAuthState('authenticated');
        void fetchServerUpdateNotice();
      }} theme={theme} onToggleTheme={toggleTheme} guestEnabled={guestEnabled} />}
      {authState === 'authenticated' && <Dashboard onUnauthorized={() => setAuthState('anonymous')} theme={theme} onToggleTheme={toggleTheme} />}
      {authState === 'guest' && <GuestDashboard theme={theme} onToggleTheme={toggleTheme} />}
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

  const content = (
    <main className="setup-shell" data-testid={mode === 'initial' ? 'setup-wizard' : 'setup-wizard-inline'}>
      <section className="setup-stage" aria-label={t('配置引导')}>
        <aside className="setup-hero">
          <div className="setup-hero-top">
            <Brand />
            <ThemeToggle theme={theme} onToggle={onToggleTheme} />
          </div>
          <div className="setup-hero-copy">
            <span className="pill good">{service?.current_scheme?.toUpperCase() ?? 'HTTP'}</span>
            <h1>{mode === 'initial' ? t('首次配置') : t('配置引导')}</h1>
            <p>{service ? `${service.current_addr} · ${service.current_scheme.toUpperCase()}` : t('初始化服务访问参数')}</p>
          </div>
          <div className="setup-hero-facts">
            <div>
              <span>{t('访问端口')}</span>
              <strong>{port || '-'}</strong>
            </div>
            <div>
              <span>{t('界面语言')}</span>
              <strong>{languages.find((item) => item.code === selectedLanguage)?.nativeLabel ?? selectedLanguage}</strong>
            </div>
            <div>
              <span>HTTPS</span>
              <strong>{certificateName || service?.cert_not_after ? t('证书已配置') : t('可选')}</strong>
            </div>
          </div>
        </aside>

        <form className="setup-panel" onSubmit={submit}>
          <div className="setup-title">
            <span>{t('凭据、端口、语言和 HTTPS 证书')}</span>
            <h2>{t('保存配置')}</h2>
            <p>{mode === 'initial' ? t('设置服务端访问方式后即可进入控制台') : t('此前配置已预填，可只修改需要变更的项目')}</p>
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
      </section>
    </main>
  );

  return mode === 'authenticated' ? createPortal(content, document.body) : content;
}

function Login({ onSuccess, theme, onToggleTheme, guestEnabled }: { onSuccess: () => void; theme: Theme; onToggleTheme: () => void; guestEnabled: boolean }) {
  const { t } = useI18n();
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

  function enterGuest() {
    window.location.assign('/guest');
  }

  return (
    <main className="login-shell">
      <form className="login-panel" onSubmit={submit}>
        <div className="login-head">
          <Brand />
          <ThemeToggle theme={theme} onToggle={onToggleTheme} />
        </div>
        <h1>{t('登录面板')}</h1>
        <p>{t('登录后记住当前设备 30 天')}</p>
        <label>
          {t('密码')}
          <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="current-password" />
        </label>
        <button className="primary" disabled={loading}>
          <LogIn size={18} />
          {loading ? t('登录中') : t('登录')}
        </button>
        {guestEnabled && (
          <button className="secondary action-button guest-login-button" type="button" onClick={enterGuest}>
            <Activity size={18} />
            {t('访客访问')}
          </button>
        )}
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
    retry: 6,
    retryDelay: (attempt) => Math.min(500 * 2 ** attempt, 3000)
  });
  const stats = useQuery({
    queryKey: ['stats', 24],
    queryFn: () => getStats(24),
    enabled: overview.isSuccess,
    refetchInterval: 30000
  });
  const cachedUpdate = readCachedUpdateStatus();
  const updatePollInterval = overview.data?.service.auto_update_enabled ? autoUpdateStatusTTL : updateStatusCacheTTL;
  const update = useQuery({
    queryKey: ['update-status'],
    queryFn: async () => {
      const next = await getUpdateStatus();
      storeCachedUpdateStatus(next);
      return next;
    },
    initialData: cachedUpdate?.status,
    initialDataUpdatedAt: cachedUpdate ? new Date(cachedUpdate.cached_at).getTime() : undefined,
    enabled: overview.isSuccess && !hasPendingUpdate(),
    staleTime: updatePollInterval,
    refetchInterval: hasPendingUpdate() ? false : updatePollInterval,
    refetchOnWindowFocus: false,
    retry: false
  });

  useEffect(() => {
    if (overview.error instanceof Error && overview.error.message.includes('login')) {
      onUnauthorized();
    }
  }, [overview.error, onUnauthorized]);

  const data = overview.data;
  const statRows = stats.data?.stats ?? [];
  const updateAttention = Boolean(update.data?.supported && !update.data.failed && (update.data.available || update.data.binary_outdated));
  const settingsNavClass = [view === 'settings' ? 'active' : '', updateAttention && view !== 'settings' ? 'has-notice' : ''].filter(Boolean).join(' ');
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
          <button className={settingsNavClass} onClick={() => setView('settings')}><Settings size={17} />设置</button>
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
    <button className="icon-button theme-toggle" type="button" onClick={onToggle} title={theme === 'dark' ? '切换浅色' : '切换深色'} data-testid="theme-toggle">
      {theme === 'dark' ? <Sun size={18} /> : <Moon size={18} />}
    </button>
  );
}

function GuestDashboard({ theme, onToggleTheme }: { theme: Theme; onToggleTheme: () => void }) {
  const { t } = useI18n();
  const overview = useQuery({
    queryKey: ['guest-overview'],
    queryFn: getGuestOverview,
    refetchInterval: 10000,
    retry: 2
  });
  const data = overview.data;
  return (
    <div className="app guest-app">
      <aside className="sidebar">
        <Brand />
        <nav>
          <button className="active"><Activity size={17} />{t('访客总览')}</button>
        </nav>
      </aside>
      <main className="content">
        <header className="topbar">
          <div>
            <h1>{t('访客总览')}</h1>
            <p>{data ? t('服务端时间 {time}', { time: new Date(data.server_time).toLocaleString() }) : t('等待服务端数据')}</p>
          </div>
          <div className="top-actions">
            <ThemeToggle theme={theme} onToggle={onToggleTheme} />
            <button className="icon-button" onClick={() => overview.refetch()} title={t('刷新')}>
              <RefreshCw size={18} />
            </button>
            <button className="icon-button" onClick={() => window.location.assign('/')} title={t('登录')}>
              <LogIn size={18} />
            </button>
          </div>
        </header>
        {overview.error && <div className="banner danger">{overview.error instanceof Error ? overview.error.message : 'guest access failed'}</div>}
        <div className="view-shell" data-view="guest">
          <OverviewPage data={data} statRows={[]} theme={theme} guest />
        </div>
      </main>
    </div>
  );
}

function OverviewPage({ data, statRows, theme, guest = false }: { data?: Overview; statRows: GPUStats[]; theme: Theme; guest?: boolean }) {
  const gpus = data?.latest_gpus ?? [];
  const aggregateSeries = useAggregateSeries(gpus, guest);
  const devices = data?.devices ?? [];
  const hotCount = gpus.filter((item) => (item.gpu.temperature_celsius ?? 0) >= 80).length;
  const busyCount = gpus.filter((item) => (item.gpu.utilization_gpu_percent ?? 0) >= 80).length;
  const onlineText = data ? `${data.online_device_count}/${data.device_count}` : '-';
  const gpuCountText = data ? String(data.gpu_count) : '-';
  const busyText = data ? String(busyCount) : '-';
  const hotText = data ? String(hotCount) : '-';
  const powerValue = data?.power_draw_watts;
  const memorySpark = aggregateSeries.ready ? aggregateSeries.memory : [];
  const powerSpark = aggregateSeries.ready ? aggregateSeries.power : [];

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
          <FleetKPI label="总显存用量" value={fmtMemoryG(data?.memory_used_bytes, data?.memory_total_bytes)} spark={{ label: '总显存用量', samples: memorySpark, max: data?.memory_total_bytes ?? maxSeries(memorySpark.map((sample) => sample.value), 1), formatValue: fmtBytes, tone: 'good' }} />
          <FleetKPI label="总功耗" value={typeof powerValue === 'number' ? watts(powerValue) : '-'} tone={(powerValue ?? 0) > 0 ? 'accent' : 'good'} spark={{ label: '总功耗', samples: powerSpark, max: maxSeries(powerSpark.map((sample) => sample.value), Math.max(powerValue ?? 1, 1)), formatValue: watts, tone: 'accent' }} />
        </div>
      </section>

      <section className="overview-layout">
        <FleetBoard items={gpus} devices={devices} guest={guest} />
        <div className="overview-side">
          <FleetUtilPanel items={gpus} theme={theme} />
          <DevicePanel data={data} />
        </div>
      </section>

      {!guest && (
        <section className="overview-secondary">
          <ProcessPanel items={data?.latest_processes ?? []} devices={devices} />
          <StatsPanel statRows={statRows} devices={devices} />
        </section>
      )}
    </>
  );
}

function GPUDetailPage({ data, statRows, theme }: { data?: Overview; statRows: GPUStats[]; theme: Theme }) {
  const gpus = data?.latest_gpus ?? [];
  const deviceMap = useMemo(() => new Map((data?.devices ?? []).map((device) => [device.id, device])), [data?.devices]);
  const aggregateSeries = useAggregateSeries(gpus);
  const powerValue = data?.power_draw_watts;
  const utilizationSpark = aggregateSeries.ready ? aggregateSeries.utilization : [];
  const memorySpark = aggregateSeries.ready ? aggregateSeries.memory : [];
  const powerSpark = aggregateSeries.ready ? aggregateSeries.power : [];

  return (
    <>
      <section className="stat-grid">
        <Metric icon={<MonitorUp />} label="在线设备" value={`${data?.online_device_count ?? 0} / ${data?.device_count ?? 0}`} />
        <Metric icon={<Cpu />} label="GPU 数量" value={String(data?.gpu_count ?? 0)} />
        <Metric icon={<Gauge />} label="平均利用率" value={pct(data?.average_utilization ?? 0)} spark={{ label: '平均利用率', samples: utilizationSpark, max: 100, formatValue: pct, tone: 'accent' }} />
        <Metric icon={<Database />} label="总显存用量" value={fmtMemoryG(data?.memory_used_bytes, data?.memory_total_bytes)} spark={{ label: '总显存用量', samples: memorySpark, max: data?.memory_total_bytes ?? maxSeries(memorySpark.map((sample) => sample.value), 1), formatValue: fmtBytes, tone: 'good' }} />
        <Metric icon={<Power />} label="总功耗" value={watts(data?.power_draw_watts ?? 0)} tone={(data?.power_draw_watts ?? 0) > 0 ? 'accent' : 'good'} spark={{ label: '总功耗', samples: powerSpark, max: maxSeries(powerSpark.map((sample) => sample.value), Math.max(powerValue ?? 1, 1)), formatValue: watts, tone: 'accent' }} />
      </section>

      <section className="main-grid">
        <div className="gpu-main-column">
          <div className="panel">
            <div className="panel-head">
              <h2>GPU 详细状态</h2>
              <span>{data?.latest_gpus.length ?? 0}</span>
            </div>
            <div className="gpu-grid">
              {(data?.latest_gpus ?? []).map((item) => <GPUCard key={`${item.device_id}-${item.gpu.gpu_id}`} item={item} device={deviceMap.get(item.device_id)} />)}
            </div>
            <UtilChart items={data?.latest_gpus ?? []} theme={theme} />
          </div>
          <StatsPanel statRows={statRows} devices={data?.devices ?? []} />
        </div>
        <div className="stack">
          <DevicePanel data={data} />
          <ProcessPanel items={data?.latest_processes ?? []} devices={data?.devices ?? []} />
        </div>
      </section>
    </>
  );
}

function FleetKPI({ label, value, tone, spark }: { label: string; value: string; tone?: 'good' | 'warn' | 'bad' | 'accent'; spark?: { label: string; samples: Array<{ value: number; timestamp?: string }>; max: number; formatValue: (value?: number) => string; tone?: TrendTone } }) {
  return (
    <div className={`fleet-kpi ${tone ?? ''} ${spark ? 'with-spark' : ''}`}>
      <div className="fleet-kpi-value">
        <span>{label}</span>
        <strong>{value}</strong>
      </div>
      {spark && (
        <Sparkline samples={spark.samples} max={spark.max} label={spark.label} formatValue={spark.formatValue} className={`fleet-kpi-spark ${spark.tone ?? 'accent'}`} />
      )}
    </div>
  );
}

function FleetBoard({ items, devices, guest = false }: { items: StoredGPU[]; devices: Device[]; guest?: boolean }) {
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
          <FleetGPUCard item={item} device={device} health={health} guest={guest} key={`${item.device_id}-${item.gpu.gpu_id}`} />
        ))}
        {cards.length === 0 && <p className="empty">暂无 GPU 上报</p>}
      </div>
    </section>
  );
}

function FleetGPUCard({ item, device, health, guest = false }: { item: StoredGPU; device?: Device; health: ReturnType<typeof gpuHealth>; guest?: boolean }) {
  const { language } = useI18n();
  const gpu = item.gpu;
  const util = gpu.utilization_gpu_percent;
  const mem = memoryUsagePercent(item);
  const powerLimit = gpu.power_limit_watts ?? gpu.power_enforced_limit_watts;
  const deviceColor = deviceBorderColor(item.device_id);
  const offlineText = offlineMaskText(item, device, language);
  const series = useQuery({
    queryKey: gpuSeriesQueryKey(item.device_id, gpu.gpu_id, 1, guest),
    queryFn: () => guest ? getGuestGPUSeries(item.device_id, gpu.gpu_id, 1) : getGPUSeries(item.device_id, gpu.gpu_id, 1),
    staleTime: 20_000,
    refetchInterval: 30000,
    retry: false,
    placeholderData: (previous) => previous
  });
  const points = mergeSeriesWithLatest(series.data ?? [], item);

  return (
    <article
      className={`fleet-gpu-card ${health.tone}`}
      data-testid="fleet-gpu-card"
      data-device-id={item.device_id}
      data-device-color={deviceColor}
      style={{ '--device-color': deviceColor } as React.CSSProperties}
    >
      {health.tone === 'offline' && <OfflineMask text={offlineText} />}
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
      <TrendTile label="利用率" value={pct(util)} caption={compact && gpu.sm_clock_mhz ? mhz(gpu.sm_clock_mhz).replace(' MHz', '') : gpu.sm_clock_mhz ? mhz(gpu.sm_clock_mhz) : '最近 1 小时'} values={points.map((point) => point.utilization_gpu_percent)} timestamps={timestamps} max={100} tone={metricTone(util, 70, 92)} formatValue={pct} />
      <TrendTile label="显存" value={memValue} caption={compact ? fmtBytes(gpu.memory_total_bytes) : `总量 ${fmtBytes(gpu.memory_total_bytes)}`} values={points.map((point) => point.memory_total_bytes ? (point.memory_used_bytes / point.memory_total_bytes) * 100 : undefined)} timestamps={timestamps} max={100} tone={metricTone(mem, 75, 92)} formatValue={pct} />
      <TrendTile label="温度" value={temp(gpu.temperature_celsius)} caption={tempToneText(gpu.temperature_celsius)} values={points.map((point) => point.temperature_celsius)} timestamps={timestamps} max={100} tone={metricTone(gpu.temperature_celsius, 80, 88)} formatValue={temp} />
      <TrendTile label="功耗" value={powerValue} caption={powerLimit ? (compact ? watts(powerLimit) : `上限 ${watts(powerLimit)}`) : gpu.pstate || '-'} values={points.map((point) => point.power_draw_watts)} timestamps={timestamps} max={powerLimit || maxSeries(points.map((point) => point.power_draw_watts), 200)} tone={metricTone(powerLimit && gpu.power_draw_watts ? (gpu.power_draw_watts / powerLimit) * 100 : undefined, 78, 95)} formatValue={watts} />
    </div>
  );
}

function TrendTile({ label, value, caption, values, timestamps, max, tone, formatValue }: { label: string; value: string; caption: string; values: Array<number | undefined>; timestamps: string[]; max: number; tone: TrendTone; formatValue: (value?: number) => string }) {
  const [valueHover, setValueHover] = useState(false);
  const clean: Array<{ value: number; timestamp?: string }> = [];
  values.forEach((item, index) => {
    if (typeof item === 'number' && Number.isFinite(item)) {
      clean.push({ value: item, timestamp: timestamps[index] });
    }
  });
  const latest = clean[clean.length - 1];
  return (
    <div className={`trend-tile ${tone}`} data-testid="gpu-trend-tile">
      <div className="trend-head">
        <div>
          <span>{label}</span>
          <strong onPointerEnter={() => setValueHover(true)} onPointerLeave={() => setValueHover(false)}>{value}</strong>
        </div>
        <p>{caption}</p>
      </div>
      {valueHover && latest && (
        <div className="spark-tooltip trend-value-tooltip" data-testid="spark-tooltip">
          <span>{label}</span>
          <strong>{formatValue(latest.value)}</strong>
          <small>{latest.timestamp ? fmtDateTime(latest.timestamp) : '-'}</small>
        </div>
      )}
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

function offlineMaskText(item: StoredGPU, device: Device | undefined, language: AppLanguage) {
  const offlineAt = device?.last_seen_at || device?.last_sample_at || item.timestamp;
  const prefix = language === 'en-US' ? 'Offline' : '离线';
  return offlineAt ? `${prefix} · ${timeAgo(offlineAt, language)}` : prefix;
}

function OfflineMask({ text }: { text: string }) {
  const [title, detail] = text.split(' · ');
  return (
    <div className="offline-mask">
      <strong>{title}</strong>
      {detail && <small>{detail}</small>}
    </div>
  );
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

type AggregateSeriesData = {
  utilization: Array<{ value: number; timestamp?: string }>;
  memory: Array<{ value: number; timestamp?: string }>;
  power: Array<{ value: number; timestamp?: string }>;
};

type AggregateSeries = AggregateSeriesData & {
  ready: boolean;
};

function useAggregateSeries(items: StoredGPU[], guest = false): AggregateSeries {
  const query = useQueryClient();
  const keys = useMemo(() => items.map((item) => `${item.device_id}/${item.gpu.gpu_id}`).sort(), [items]);
  const series = useQuery({
    queryKey: ['aggregate-gpu-series', guest ? 'guest' : 'admin', keys],
    queryFn: async () => {
      const batches = await Promise.all(items.map(async (item) => ({
        item,
        points: await query.fetchQuery({
          queryKey: gpuSeriesQueryKey(item.device_id, item.gpu.gpu_id, 1, guest),
          queryFn: () => guest ? getGuestGPUSeries(item.device_id, item.gpu.gpu_id, 1) : getGPUSeries(item.device_id, item.gpu.gpu_id, 1),
          staleTime: 20_000
        })
      })));
      return buildAggregateSeries(batches);
    },
    enabled: items.length > 0,
    staleTime: 20_000,
    refetchInterval: 30000,
    retry: false,
    placeholderData: (previous) => previous
  });
  const data = appendCurrentAggregateSample(series.data ?? { utilization: [], memory: [], power: [] }, items);
  return series.data || items.length > 0 ? { ...data, ready: true } : { utilization: [], memory: [], power: [], ready: false };
}

function gpuSeriesQueryKey(deviceID: string, gpuID: string, hours: number, guest = false) {
  return ['gpu-series', guest ? 'guest' : 'admin', deviceID, gpuID, hours] as const;
}

function buildAggregateSeries(batches: Array<{ item: StoredGPU; points: GPUSeriesPoint[] }>): AggregateSeriesData {
  const buckets = new Map<number, { timestamp?: string; utilizationTotal: number; utilizationCount: number; memory: number; power: number }>();
  for (const { points } of batches) {
    const perGPUBuckets = new Map<number, GPUSeriesPoint>();
    for (const point of points) {
      const time = new Date(point.timestamp).getTime();
      if (!Number.isFinite(time)) continue;
      const bucket = aggregateBucketTime(time);
      const previous = perGPUBuckets.get(bucket);
      if (!previous || new Date(point.timestamp).getTime() >= new Date(previous.timestamp).getTime()) {
        perGPUBuckets.set(bucket, point);
      }
    }
    for (const [bucket, point] of perGPUBuckets) {
      const row = buckets.get(bucket) ?? { timestamp: point.timestamp, utilizationTotal: 0, utilizationCount: 0, memory: 0, power: 0 };
      if (!row.timestamp || new Date(point.timestamp).getTime() > new Date(row.timestamp).getTime()) {
        row.timestamp = point.timestamp;
      }
      if (typeof point.utilization_gpu_percent === 'number' && Number.isFinite(point.utilization_gpu_percent)) {
        row.utilizationTotal += point.utilization_gpu_percent;
        row.utilizationCount += 1;
      }
      if (typeof point.memory_used_bytes === 'number' && Number.isFinite(point.memory_used_bytes)) {
        row.memory += point.memory_used_bytes;
      }
      if (typeof point.power_draw_watts === 'number' && Number.isFinite(point.power_draw_watts)) {
        row.power += point.power_draw_watts;
      }
      buckets.set(bucket, row);
    }
  }
  const rows = Array.from(buckets.entries()).sort(([left], [right]) => left - right).map(([, row]) => row);
  return {
    utilization: rows.map((row) => ({ value: row.utilizationCount ? row.utilizationTotal / row.utilizationCount : 0, timestamp: row.timestamp })),
    memory: rows.map((row) => ({ value: row.memory, timestamp: row.timestamp })),
    power: rows.map((row) => ({ value: row.power, timestamp: row.timestamp }))
  };
}

function mergeSeriesWithLatest(points: GPUSeriesPoint[], item: StoredGPU): GPUSeriesPoint[] {
  const latest = latestPointFromItem(item);
  const latestTime = new Date(latest.timestamp).getTime();
  if (!Number.isFinite(latestTime)) return points;
  const out = [...points];
  const sameIndex = out.findIndex((point) => point.timestamp === latest.timestamp);
  if (sameIndex >= 0) {
    out[sameIndex] = latest;
    return out.sort((left, right) => new Date(left.timestamp).getTime() - new Date(right.timestamp).getTime());
  }
  const last = out[out.length - 1];
  const lastTime = last ? new Date(last.timestamp).getTime() : Number.NEGATIVE_INFINITY;
  if (!Number.isFinite(lastTime) || latestTime > lastTime) {
    out.push(latest);
    return out;
  }
  return out;
}

function latestPointFromItem(item: StoredGPU): GPUSeriesPoint {
  return {
    timestamp: item.timestamp,
    utilization_gpu_percent: item.gpu.utilization_gpu_percent,
    memory_used_bytes: item.gpu.memory_used_bytes,
    memory_total_bytes: item.gpu.memory_total_bytes,
    temperature_celsius: item.gpu.temperature_celsius,
    power_draw_watts: item.gpu.power_draw_watts
  };
}

function appendCurrentAggregateSample(data: AggregateSeriesData, items: StoredGPU[]): AggregateSeriesData {
  if (!items.length) return data;
  const current = aggregateCurrentSample(items);
  if (!current.timestamp) return data;
  return {
    utilization: appendOrReplaceAggregatePoint(data.utilization, current.timestamp, current.utilization),
    memory: appendOrReplaceAggregatePoint(data.memory, current.timestamp, current.memory),
    power: appendOrReplaceAggregatePoint(data.power, current.timestamp, current.power)
  };
}

function aggregateCurrentSample(items: StoredGPU[]) {
  let timestamp = '';
  let utilizationTotal = 0;
  let utilizationCount = 0;
  let memory = 0;
  let power = 0;
  for (const item of items) {
    if (!timestamp || new Date(item.timestamp).getTime() > new Date(timestamp).getTime()) {
      timestamp = item.timestamp;
    }
    if (typeof item.gpu.utilization_gpu_percent === 'number' && Number.isFinite(item.gpu.utilization_gpu_percent)) {
      utilizationTotal += item.gpu.utilization_gpu_percent;
      utilizationCount += 1;
    }
    if (typeof item.gpu.memory_used_bytes === 'number' && Number.isFinite(item.gpu.memory_used_bytes)) {
      memory += item.gpu.memory_used_bytes;
    }
    if (typeof item.gpu.power_draw_watts === 'number' && Number.isFinite(item.gpu.power_draw_watts)) {
      power += item.gpu.power_draw_watts;
    }
  }
  return {
    timestamp,
    utilization: utilizationCount ? utilizationTotal / utilizationCount : 0,
    memory,
    power
  };
}

function appendOrReplaceAggregatePoint(samples: Array<{ value: number; timestamp?: string }>, timestamp: string, value: number) {
  const next = [...samples];
  const currentTime = new Date(timestamp).getTime();
  if (!Number.isFinite(currentTime)) return next;
  const currentBucket = aggregateBucketTime(currentTime);
  const replaceIndex = next.findIndex((sample) => {
    if (!sample.timestamp) return false;
    const time = new Date(sample.timestamp).getTime();
    return Number.isFinite(time) && aggregateBucketTime(time) === currentBucket;
  });
  const sample = { value, timestamp };
  if (replaceIndex >= 0) {
    next[replaceIndex] = sample;
    return next.sort((left, right) => new Date(left.timestamp ?? 0).getTime() - new Date(right.timestamp ?? 0).getTime());
  }
  const last = next[next.length - 1];
  const lastTime = last?.timestamp ? new Date(last.timestamp).getTime() : Number.NEGATIVE_INFINITY;
  if (!Number.isFinite(lastTime) || currentTime >= lastTime) {
    next.push(sample);
  }
  return next;
}

function aggregateBucketTime(time: number) {
  return Math.floor(time / 60000) * 60000;
}

function Sparkline({ samples, max, label, formatValue, className = '' }: { samples: Array<{ value: number; timestamp?: string }>; max: number; label: string; formatValue: (value?: number) => string; className?: string }) {
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);
  const width = 180;
  const height = 58;
  const pad = 4;
  const clean = useMemo(() => prepareSparklineSamples(samples, width - pad * 2), [samples]);
  const hasLine = clean.length >= 2;
  const cappedMax = Math.max(1, max);
  const pointData = clean.map((sample, index) => {
    const x = clean.length === 1 ? width - pad : pad + (index / (clean.length - 1)) * (width - pad * 2);
    const y = height - pad - (Math.max(0, Math.min(cappedMax, sample.value)) / cappedMax) * (height - pad * 2);
    return { ...sample, x, y };
  });
  const line = smoothSparklinePath(pointData);
  const area = sparklineAreaPath(pointData, height - pad);
  const active = hasLine && hoverIndex !== null ? pointData[hoverIndex] : undefined;

  function onPointerMove(event: React.PointerEvent<HTMLDivElement>) {
    if (!hasLine) return;
    const rect = event.currentTarget.getBoundingClientRect();
    const ratio = rect.width > 0 ? (event.clientX - rect.left) / rect.width : 1;
    const index = Math.max(0, Math.min(clean.length - 1, Math.round(ratio * (clean.length - 1))));
    setHoverIndex(index);
  }

  return (
    <div className={`sparkline-wrap ${className}`} onPointerMove={onPointerMove} onPointerLeave={() => setHoverIndex(null)}>
      <svg className="sparkline" viewBox={`0 0 ${width} ${height}`} role="img" aria-label={`${label} 历史趋势图`} preserveAspectRatio="none">
        <polyline className="spark-grid" points={`${pad},${height - pad} ${width - pad},${height - pad}`} />
        {hasLine && (
          <>
            <path className="spark-area" d={area} />
            <path className="spark-line" d={line} />
          </>
        )}
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

function prepareSparklineSamples(samples: Array<{ value: number; timestamp?: string }>, targetPixels: number) {
  const clean = samples.filter((sample) => typeof sample.value === 'number' && Number.isFinite(sample.value));
  const maxPoints = Math.max(24, Math.floor(targetPixels));
  if (clean.length <= maxPoints) return clean;
  const buckets = new Map<number, { value: number; timestamp?: string; count: number }>();
  clean.forEach((sample, index) => {
    const bucketIndex = Math.min(maxPoints - 1, Math.floor((index / Math.max(1, clean.length - 1)) * maxPoints));
    const bucket = buckets.get(bucketIndex) ?? { value: 0, timestamp: sample.timestamp, count: 0 };
    bucket.value += sample.value;
    bucket.count += 1;
    bucket.timestamp = sample.timestamp;
    buckets.set(bucketIndex, bucket);
  });
  return Array.from(buckets.entries())
    .sort(([left], [right]) => left - right)
    .map(([, bucket]) => ({ value: bucket.value / bucket.count, timestamp: bucket.timestamp }));
}

function smoothSparklinePath(points: Array<{ x: number; y: number }>) {
  if (!points.length) return '';
  if (points.length === 1) return `M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)}`;
  const parts = [`M ${points[0].x.toFixed(1)} ${points[0].y.toFixed(1)}`];
  for (let index = 1; index < points.length; index += 1) {
    const previous = points[index - 1];
    const current = points[index];
    const midX = (previous.x + current.x) / 2;
    parts.push(`C ${midX.toFixed(1)} ${previous.y.toFixed(1)}, ${midX.toFixed(1)} ${current.y.toFixed(1)}, ${current.x.toFixed(1)} ${current.y.toFixed(1)}`);
  }
  return parts.join(' ');
}

function sparklineAreaPath(points: Array<{ x: number; y: number }>, baseline: number) {
  if (points.length < 2) return '';
  return `${smoothSparklinePath(points)} L ${points[points.length - 1].x.toFixed(1)} ${baseline.toFixed(1)} L ${points[0].x.toFixed(1)} ${baseline.toFixed(1)} Z`;
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

function Metric({ icon, label, value, tone, spark }: { icon: React.ReactNode; label: string; value: string; tone?: string; spark?: { label: string; samples: Array<{ value: number; timestamp?: string }>; max: number; formatValue: (value?: number) => string; tone?: TrendTone } }) {
  return (
    <article className={`metric ${tone ?? ''} ${spark ? 'with-spark' : ''}`}>
      <div className="metric-copy">
        <div className="metric-icon">{icon}</div>
        <p>{label}</p>
        <strong>{value}</strong>
      </div>
      {spark && (
        <Sparkline samples={spark.samples} max={spark.max} label={spark.label} formatValue={spark.formatValue} className={`metric-spark ${spark.tone ?? 'accent'}`} />
      )}
    </article>
  );
}

function GPUCard({ item, device }: { item: StoredGPU; device?: Device }) {
  const { language } = useI18n();
  const gpu = item.gpu;
  const health = gpuHealth(item, device);
  const pcie = [gpu.pcie_link_generation ? `Gen ${gpu.pcie_link_generation}` : '', gpu.pcie_link_width ? `x${gpu.pcie_link_width}` : ''].filter(Boolean).join(' ');
  const pcieMax = [gpu.pcie_link_generation_max ? `Gen ${gpu.pcie_link_generation_max}` : '', gpu.pcie_link_width_max ? `x${gpu.pcie_link_width_max}` : ''].filter(Boolean).join(' ');
  const deviceColor = deviceBorderColor(item.device_id);
  const offlineText = offlineMaskText(item, device, language);
  const series = useQuery({
    queryKey: gpuSeriesQueryKey(item.device_id, gpu.gpu_id, 1),
    queryFn: () => getGPUSeries(item.device_id, gpu.gpu_id, 1),
    staleTime: 20_000,
    refetchInterval: 30000,
    retry: false,
    placeholderData: (previous) => previous
  });
  const points = mergeSeriesWithLatest(series.data ?? [], item);
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
    <article className={`gpu-card ${health.tone}`} data-device-id={item.device_id} data-device-color={deviceColor} style={{ '--device-color': deviceColor } as React.CSSProperties}>
      {health.tone === 'offline' && <OfflineMask text={offlineText} />}
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
  const { t } = useI18n();
  const isGuest = Boolean(data?.guest);
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
            {!isGuest && <p>{[device.hostname, device.os, device.agent_version].filter(Boolean).join(' · ') || device.id}</p>}
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

  return createPortal(
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
    </div>,
    document.body
  );
}

function UpdateNoticeDialog({ notice, onClose }: { notice?: CompletedUpdateNotice; onClose: () => void }) {
  const { language, t } = useI18n();

  if (!notice) return null;
  const isCertificate = notice.kind === 'certificate';
  const isRestart = notice.kind === 'restart';
  const isAutomatic = notice.kind === 'auto_update';
  const isUpdate = !isCertificate && !isRestart;
  const from = shortHash(notice.previous_commit);
  const to = shortHash(notice.current_commit || notice.target_commit);
  const versionText = notice.current_version ? `v${notice.current_version}` : '-';
  const title = isCertificate ? t('HTTPS 证书已启用') : isRestart ? t('服务已重启') : isAutomatic ? t('自动更新已完成') : t('版本已更新');
  const body = isCertificate ? t('HTTPS 证书已保存，服务端已自动重启并刷新页面。') : isRestart ? t('服务端已重启并刷新页面。') : isAutomatic ? t('服务端已自动完成更新并重启。') : t('服务端已自动重启并刷新页面。');
  const summary = (language === 'en-US' && notice.summary_en?.length ? notice.summary_en : notice.summary)?.filter(Boolean) ?? [];
  const updateTime = notice.completed_at || notice.updated_at;

  return createPortal(
    <div className="modal-backdrop" role="presentation">
      <section className="confirm-dialog update-notice-dialog" role="dialog" aria-modal="true" aria-labelledby="update-notice-title" data-testid="update-notice-dialog">
        <div className="confirm-icon"><CheckCircle2 size={22} /></div>
        <div className="confirm-copy">
          <span>{notice.product || 'GPUFleet'}</span>
          <h2 id="update-notice-title">{title}</h2>
          <p>{body}</p>
        </div>
        {isUpdate && <div className="confirm-target update-notice-grid">
          <div>
            <span>{t('版本')}</span>
            <strong>{versionText}</strong>
          </div>
          <div>
            <span>{t('提交')}</span>
            <strong title={notice.current_commit || notice.target_commit}>{from !== '-' && to !== '-' ? `${from} -> ${to}` : to}</strong>
          </div>
          <div>
            <span>{t('更新时间')}</span>
            <strong>{fmtDateTime(updateTime)}</strong>
          </div>
        </div>}
        {isUpdate && (
          <div className="confirm-target update-summary">
            <span>{t('更新内容')}</span>
            <ul>
              {(summary.length ? summary : [t('无更新说明')]).map((item) => <li key={item}>{item}</li>)}
            </ul>
          </div>
        )}
        <div className="confirm-actions">
          <button className="primary compact" type="button" onClick={onClose}>
            <CheckCircle2 size={16} />
            {t('知道了')}
          </button>
        </div>
      </section>
    </div>,
    document.body
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

function ProcessPanel({ items, devices }: { items: StoredProcess[]; devices: Device[] }) {
  const deviceByID = new Map(devices.map((device) => [device.id, device]));
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
            <p>{deviceName(deviceByID.get(item.device_id), item.device_id)} · PID {item.process.pid} · {item.process.gpu_id || '-'}</p>
          </div>
          <span className="pill">{fmtBytes(item.process.used_memory_bytes)}</span>
        </div>
      ))}
      {items.length === 0 && <p className="empty">暂无 GPU 进程快照</p>}
    </section>
  );
}

function StatsPanel({ statRows, devices }: { statRows: GPUStats[]; devices: Device[] }) {
  const query = useQueryClient();
  const deviceByID = new Map(devices.map((device) => [device.id, device]));
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  useEffect(() => {
    const activeRows = statRows.slice(0, 8);
    const timers = activeRows.map((row, index) => window.setTimeout(() => {
      void prefetchStatsSeries(query, row);
    }, 2500 + index * 220));
    return () => timers.forEach((timer) => window.clearTimeout(timer));
  }, [query, statRows]);
  useEffect(() => {
    const current = new Set(statRows.map((row) => statsRowKey(row)));
    setExpanded((previous) => {
      const next = new Set(Array.from(previous).filter((key) => current.has(key)));
      return next.size === previous.size ? previous : next;
    });
  }, [statRows]);
  return (
    <section className="panel">
      <div className="panel-head">
        <h2>24 小时统计</h2>
        <span>{statRows.length}</span>
      </div>
      <div className="stats-table">
        {statRows.map((row) => {
          const key = statsRowKey(row);
          const open = expanded.has(key);
          return (
            <div className="stats-expand-row" key={key}>
              <button
                className={`table-row stats-row-trigger ${open ? 'active' : ''}`}
                type="button"
                onClick={() => {
                  setExpanded((previous) => {
                    const next = new Set(previous);
                    if (next.has(key)) {
                      next.delete(key);
                    } else {
                      next.add(key);
                    }
                    return next;
                  });
                }}
                onPointerEnter={() => prefetchStatsSeries(query, row)}
                aria-expanded={open}
              >
                <div>
                  <strong>{row.gpu_name || row.gpu_id}</strong>
                  <p>{deviceName(deviceByID.get(row.device_id), row.device_id)} · {row.gpu_id} · {row.sample_count} samples</p>
                </div>
                <span>{pct(row.average_utilization_percent)}</span>
                <span>{pct(row.idle_sample_percent)} idle</span>
                <span>{row.peak_temperature_celsius ? `${Math.round(row.peak_temperature_celsius)}°C` : '-'}</span>
                <span>{row.peak_power_draw_watts ? `${row.peak_power_draw_watts.toFixed(1)} W` : '-'}</span>
              </button>
              {open && <StatsTrendCard row={row} />}
            </div>
          );
        })}
      </div>
    </section>
  );
}

function statsRowKey(row: Pick<GPUStats, 'device_id' | 'gpu_id'>) {
  return `${row.device_id}-${row.gpu_id}`;
}

function StatsTrendCard({ row }: { row: GPUStats }) {
  const series = useQuery({
    queryKey: statsSeriesQueryKey(row),
    queryFn: () => getGPUSeries(row.device_id, row.gpu_id, 24),
    staleTime: 30_000,
    retry: false
  });
  const points = series.data ?? [];
  const memoryValues = points.map((point) => point.memory_total_bytes ? (point.memory_used_bytes / point.memory_total_bytes) * 100 : undefined);
  return (
    <div className="stats-trend-card">
      <div className="stats-trend-head">
        <div>
          <strong>过去 24H 曲线</strong>
          <p>{row.gpu_name || row.gpu_id}</p>
        </div>
        <span>{series.isLoading ? '加载中' : `${points.length} samples`}</span>
      </div>
      <div className="gpu-detail-trend-grid stats-trend-grid">
        <TrendTile label="利用率" value={pct(row.average_utilization_percent)} caption="平均" values={points.map((point) => point.utilization_gpu_percent)} timestamps={points.map((point) => point.timestamp)} max={100} tone={metricTone(row.average_utilization_percent, 70, 92)} formatValue={pct} />
        <TrendTile label="显存" value={fmtMemoryG(row.peak_memory_used_bytes, row.memory_total_bytes)} caption="峰值" values={memoryValues} timestamps={points.map((point) => point.timestamp)} max={100} tone={metricTone(maxSeries(memoryValues, 0), 75, 92)} formatValue={pct} />
        <TrendTile label="温度" value={row.peak_temperature_celsius ? `${Math.round(row.peak_temperature_celsius)}°C` : '-'} caption="峰值" values={points.map((point) => point.temperature_celsius)} timestamps={points.map((point) => point.timestamp)} max={100} tone={metricTone(row.peak_temperature_celsius, 80, 88)} formatValue={temp} />
        <TrendTile label="功耗" value={watts(row.peak_power_draw_watts)} caption="峰值" values={points.map((point) => point.power_draw_watts)} timestamps={points.map((point) => point.timestamp)} max={maxSeries(points.map((point) => point.power_draw_watts), Math.max(row.peak_power_draw_watts ?? 1, 1))} tone={metricTone(row.peak_power_draw_watts, 240, 300)} formatValue={watts} />
      </div>
      {series.error instanceof Error && <p className="error">{series.error.message}</p>}
    </div>
  );
}

function statsSeriesQueryKey(row: Pick<GPUStats, 'device_id' | 'gpu_id'>) {
  return ['gpu-series-24h', row.device_id, row.gpu_id] as const;
}

function prefetchStatsSeries(query: QueryClient, row: Pick<GPUStats, 'device_id' | 'gpu_id'>) {
  return query.prefetchQuery({
    queryKey: statsSeriesQueryKey(row),
    queryFn: () => getGPUSeries(row.device_id, row.gpu_id, 24),
    staleTime: 30_000
  });
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
          <SettingStat label="磁盘预留" value={fmtBytes(service?.min_free_bytes ?? min)} caption={t('空闲 {value}', { value: fmtBytes(data?.disk.free_bytes) })} />
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
          <GuestSettings service={service} onDone={refreshOverview} />
          <RestartSettings service={service} />
        </div>

        <div className="settings-column settings-column-operations">
          <div className="settings-section-head">
            <div>
              <h2>维护与发布</h2>
              <p>数据库、在线更新和版本信息</p>
            </div>
          </div>
          <DatabaseSettings data={data} />
          <DiskReserveSettings data={data} onDone={refreshOverview} />
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
  const [checking, setChecking] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [waitingForRestart, setWaitingForRestart] = useState(() => hasPendingUpdate());
  const [proxyURL, setProxyURL] = useState(service?.update_proxy || '');
  const [proxyMessage, setProxyMessage] = useState('');
  const [savingProxy, setSavingProxy] = useState(false);
  const [autoUpdateEnabled, setAutoUpdateEnabled] = useState(service?.auto_update_enabled ?? true);
  const [savingAutoUpdate, setSavingAutoUpdate] = useState(false);
  const [progressStep, setProgressStep] = useState(0);
  const [updateDetail, setUpdateDetail] = useState('');
  const [detailOpen, setDetailOpen] = useState(false);
  const cachedUpdate = readCachedUpdateStatus();
  const cachedUpdateAge = cachedUpdate ? Date.now() - new Date(cachedUpdate.cached_at).getTime() : Number.POSITIVE_INFINITY;
  const updatePollInterval = autoUpdateEnabled ? autoUpdateStatusTTL : updateStatusCacheTTL;
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
    staleTime: updatePollInterval,
    refetchInterval: waitingForRestart ? false : updatePollInterval,
    refetchOnWindowFocus: false,
    refetchOnMount: !waitingForRestart && cachedUpdateAge >= updatePollInterval,
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

  useEffect(() => {
    setAutoUpdateEnabled(service?.auto_update_enabled ?? true);
  }, [service?.auto_update_enabled]);

  async function check() {
    setMessage('');
    setUpdateDetail('');
    setProgressStep(0);
    setWaitingForRestart(false);
    setChecking(true);
    try {
      const next = await getUpdateStatus(true);
      storeCachedUpdateStatus(next);
      query.setQueryData(['update-status'], next);
      setUpdateDetail(next.detail || '');
    } catch (err) {
      const detail = err instanceof Error ? err.message : 'update check failed';
      setMessage(friendlyUpdateFailure(detail, Boolean(proxyURL.trim())));
      setUpdateDetail(detail);
    } finally {
      setChecking(false);
    }
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

  async function toggleAutoUpdate(enabled: boolean) {
    setAutoUpdateEnabled(enabled);
    setProxyMessage('');
    setSavingAutoUpdate(true);
    try {
      await updateServerConfig({ auto_update_enabled: enabled });
      setProxyMessage(enabled ? '自动更新已开启' : '自动更新已关闭');
      await onDone();
      await query.invalidateQueries({ queryKey: ['update-status'] });
    } catch (err) {
      setAutoUpdateEnabled(!enabled);
      setProxyMessage(err instanceof Error ? err.message : 'auto update config failed');
    } finally {
      setSavingAutoUpdate(false);
    }
  }

  async function pull() {
    setConfirmOpen(false);
    setMessage('');
    setUpdateDetail('');
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
      const detail = err instanceof Error ? err.message : 'update failed';
      setMessage(friendlyUpdateFailure(detail, Boolean(proxyURL.trim())));
      setUpdateDetail(detail);
    } finally {
      setBusy(false);
    }
  }
  const visibleMessage = message || state.message;
  const visibleDetail = updateDetail || status?.detail || '';
  const messageIsSuccess = message ? (message.includes('已') || message.includes('正在自动重启') || message.includes('当前已经是最新版本')) : state.tone === 'good';
  const messageClass = messageIsSuccess ? 'notice update-note' : 'error update-note';

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
          <span>检查时间</span>
          <strong>{fmtDateTime(status?.checked_at)}</strong>
        </div>
      </div>

      <label className="switch-row update-auto-row">
        <input
          type="checkbox"
          checked={autoUpdateEnabled}
          disabled={savingAutoUpdate || busy}
          onChange={(event) => void toggleAutoUpdate(event.target.checked)}
        />
        <span>{autoUpdateEnabled ? '自动更新已开启' : '自动更新已关闭'}</span>
        <small>{autoUpdateEnabled ? '每 30 分钟检查一次，有更新时自动拉取、构建并重启' : '每 1 小时检查一次，有更新时在设置入口提示'}</small>
      </label>

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
        <button className="secondary" type="button" onClick={check} disabled={update.isFetching || checking || busy}>
          <RefreshCw size={16} />
          {update.isFetching || checking ? '检查中' : '检查更新'}
        </button>
        <button className="primary compact" type="button" onClick={() => setConfirmOpen(true)} disabled={!canApply}>
          <Download size={16} />
          {busy ? '更新中' : '更新'}
        </button>
      </div>
      {confirmOpen && (
        <UpdateConfirmDialog
          status={status}
          busy={busy}
          onCancel={() => setConfirmOpen(false)}
          onConfirm={pull}
        />
      )}
      {progressStep > 0 && <UpdateProgress step={progressStep} />}
      {visibleMessage && (
        <div className={`update-note-row ${messageClass}`}>
          <p>{visibleMessage}</p>
          {visibleDetail && (
            <button className="icon-button inline-help" type="button" onClick={() => setDetailOpen(true)} title="查看 Git 原始错误">
              <CircleHelp size={16} />
            </button>
          )}
        </div>
      )}
      {detailOpen && <UpdateDetailDialog detail={visibleDetail} onClose={() => setDetailOpen(false)} />}
    </article>
  );
}

function UpdateDetailDialog({ detail, onClose }: { detail: string; onClose: () => void }) {
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);
  return createPortal(
    <div className="modal-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <section className="confirm-dialog update-detail-dialog" role="dialog" aria-modal="true" aria-labelledby="update-detail-title" data-testid="update-detail-dialog">
        <div className="confirm-copy">
          <span className="confirm-icon"><CircleHelp size={22} /></span>
          <div>
            <h2 id="update-detail-title">Git 原始错误</h2>
            <p>用于诊断服务器网络、代理或 Git 上游问题。</p>
          </div>
        </div>
        <pre>{detail || '-'}</pre>
        <div className="dialog-actions">
          <button className="primary narrow" type="button" onClick={onClose}>关闭</button>
        </div>
      </section>
    </div>,
    document.body
  );
}

function UpdateConfirmDialog({ status, busy, onCancel, onConfirm }: { status?: UpdateStatus; busy: boolean; onCancel: () => void; onConfirm: () => void }) {
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onCancel();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onCancel]);

  return createPortal(
    <div className="modal-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onCancel();
    }}>
      <section className="confirm-dialog warning" role="dialog" aria-modal="true" aria-labelledby="update-confirm-title" data-testid="update-confirm-dialog">
        <div className="confirm-icon"><Download size={22} /></div>
        <div className="confirm-copy">
          <span>在线更新</span>
          <h2 id="update-confirm-title">确认更新服务端？</h2>
          <p>服务端会检查依赖、构建远端提交、执行 fast-forward 拉取，并在成功后自动重启。重启期间页面会显示进度并等待服务恢复。</p>
        </div>
        <div className="confirm-target update-notice-grid">
          <div><span>当前提交</span><strong>{shortHash(status?.local_commit)}</strong></div>
          <div><span>远端提交</span><strong>{shortHash(status?.remote_commit)}</strong></div>
          <div><span>落后</span><strong>{status?.behind ?? 0}</strong></div>
          <div><span>仓库版本</span><strong>{status?.repo_version ? `v${status.repo_version}` : '-'}</strong></div>
        </div>
        <div className="confirm-actions">
          <button className="secondary" type="button" onClick={onCancel} disabled={busy}>取消</button>
          <button className="primary compact" type="button" onClick={onConfirm} disabled={busy}>
            <Download size={16} />
            {busy ? '更新中' : '确认更新'}
          </button>
        </div>
      </section>
    </div>,
    document.body
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
  const activeLabel = stages[Math.max(0, Math.min(stages.length - 1, step - 1))];
  const rawPercent = Math.round((Math.max(1, Math.min(step, stages.length)) / stages.length) * 100);
  const percent = step >= 4 ? 99 : rawPercent;
  return createPortal(
    <div className="modal-backdrop update-progress-backdrop" role="presentation" data-testid="update-progress">
      <section className="confirm-dialog update-progress-dialog" role="status" aria-live="polite">
        <div className="update-progress-head">
          <div className="confirm-icon"><Download size={18} /></div>
          <div className="confirm-copy">
            <span>在线更新</span>
            <h2>正在拉取并重启</h2>
            <p>{activeLabel}</p>
          </div>
          <strong>{percent}%</strong>
        </div>
        <div className="update-progress-bar"><span style={{ width: `${percent}%` }} /></div>
        <div className="update-progress">
          {stages.map((label, index) => (
            <div className={index + 1 < step ? 'done' : index + 1 === step ? 'active' : ''} key={label}>
              <span>{index + 1}</span>
              <p>{label}</p>
            </div>
          ))}
        </div>
      </section>
    </div>,
    document.body
  );
}

function updateState(status?: UpdateStatus, loading = false, error = '') {
  if (loading) return { label: '检查中', tone: 'warn', message: '正在读取 Git 状态' };
  if (error) return { label: '失败', tone: 'bad', message: error };
  if (!status) return { label: '未知', tone: 'warn', message: '尚未读取更新状态' };
  if (!status.supported) return { label: '不可用', tone: 'bad', message: status.message || '服务端未运行在 Git 工作区' };
  if (status.failed) return { label: '检查失败', tone: 'bad', message: status.message || '检查 Git 上游失败' };
  if (status.dirty) return { label: '已阻止', tone: 'bad', message: '服务端工作区存在未提交改动，已阻止自动拉取' };
  if (!status.upstream) return { label: '未绑定', tone: 'warn', message: status.message || '当前分支没有 Git upstream' };
  if (status.ahead > 0 && status.behind > 0) return { label: '分叉', tone: 'bad', message: '本地和上游存在分叉，不能自动 fast-forward' };
  if (status.ahead > 0) return { label: '本地超前', tone: 'warn', message: '本地提交超前上游，面板不会执行拉取' };
  if (status.available) return { label: '有新版本', tone: 'good', message: `${status.behind} 个提交可拉取、构建并自动重启` };
  if (status.binary_outdated) return { label: '需重建', tone: 'warn', message: '运行中的服务端二进制与当前仓库不一致，可重建并自动重启' };
  return { label: '最新', tone: 'good', message: status.message || '已经是最新版本' };
}

function friendlyUpdateFailure(detail: string, hasProxy: boolean) {
  const lower = detail.toLowerCase();
  const proxyHint = hasProxy ? '请确认当前更新代理可由服务端访问。' : '请在设置页配置服务端可访问的更新代理，或检查服务器直连 GitHub 的网络。';
  if (lower.includes('gnutls') || lower.includes('handshake') || lower.includes('tls')) {
    return `在线更新失败：GitHub TLS 连接被中断。${proxyHint}`;
  }
  if (lower.includes('could not resolve host') || lower.includes('name resolution')) {
    return '在线更新失败：服务器无法解析 GitHub 域名。请检查 DNS、网络或更新代理。';
  }
  if (lower.includes('connection timed out') || lower.includes('failed to connect') || lower.includes('connection refused')) {
    return `在线更新失败：服务器连接 GitHub 超时或被拒绝。${proxyHint}`;
  }
  if (lower.includes('authentication failed') || lower.includes('could not read username')) {
    return '在线更新失败：远端仓库认证失败。请检查仓库地址、访问权限或凭据配置。';
  }
  return '在线更新失败，请查看详情并检查服务器网络、Git 上游或更新代理配置。';
}

function ProjectInfoSettings({ release, loading, error }: { release?: ReleaseInfo; loading: boolean; error: string }) {
  const { language } = useI18n();
  const [changelogOpen, setChangelogOpen] = useState(false);
  const allEntries = release?.changelog ?? [];
  const entries = allEntries.slice(0, 1);
  const versionText = release?.version ? `v${release.version}` : loading ? '加载中' : '-';
  const commitText = release?.commit && release.commit !== 'dev' ? release.commit : 'dev';
  const moreLabel = language === 'en-US' ? 'More changelog' : '更多更新记录';

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
        {entries.length > 0 ? (
          <div className="changelog-entry-list">
            {entries.map((entry) => <ChangelogEntryView entry={entry} language={language} key={`${entry.version}-${entry.date}`} />)}
            {allEntries.length > 1 && (
              <button className="secondary changelog-toggle" type="button" onClick={() => setChangelogOpen(true)}>
                <BookOpenText size={15} />
                {moreLabel}
              </button>
            )}
          </div>
        ) : <p>{error || '正在读取版本信息'}</p>}
      </div>
      {changelogOpen && <ChangelogDialog entries={allEntries} language={language} onClose={() => setChangelogOpen(false)} />}
      <a className="secondary action-button" href={release?.repository ?? repositoryURL} target="_blank" rel="noreferrer">
        <Github size={16} />
        打开 GitHub
      </a>
    </article>
  );
}

function ChangelogDialog({ entries, language, onClose }: { entries: NonNullable<ReleaseInfo['changelog']>; language: AppLanguage; onClose: () => void }) {
  const title = language === 'en-US' ? 'Changelog' : '更新记录';
  const subtitle = language === 'en-US' ? 'Complete release history from CHANGELOG.md' : '从 CHANGELOG.md 读取的完整更新记录';
  const closeTitle = language === 'en-US' ? 'Close' : '关闭';

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  return createPortal(
    <div className="modal-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <section className="confirm-dialog changelog-dialog" role="dialog" aria-modal="true" aria-labelledby="changelog-dialog-title" data-testid="changelog-dialog">
        <div className="panel-head">
          <div>
            <h2 id="changelog-dialog-title">{title}</h2>
            <p>{subtitle}</p>
          </div>
          <button className="icon-button" type="button" onClick={onClose} title={closeTitle}>×</button>
        </div>
        <div className="changelog-dialog-body">
          {entries.map((entry) => <ChangelogEntryView entry={entry} language={language} key={`${entry.version}-${entry.date}`} />)}
        </div>
      </section>
    </div>,
    document.body
  );
}

function ChangelogEntryView({ entry, language }: { entry: NonNullable<ReleaseInfo['changelog']>[number]; language: AppLanguage }) {
  const title = language === 'en-US' ? entry.title_en || entry.title : entry.title;
  const added = language === 'en-US' ? entry.added_en || entry.added : entry.added;
  const changed = language === 'en-US' ? entry.changed_en || entry.changed : entry.changed;
  const security = language === 'en-US' ? entry.security_en || entry.security : entry.security;
  const fixed = language === 'en-US' ? entry.fixed_en || entry.fixed : entry.fixed;
  const labels = language === 'en-US'
    ? { added: 'Added', changed: 'Changed', security: 'Security', fixed: 'Fixed' }
    : { added: '新增', changed: '变更', security: '安全', fixed: '修复' };
  return (
    <div className="changelog-entry">
      <div>
        <strong>v{entry.version}</strong>
        <span>{entry.date}</span>
      </div>
      <p>{title}</p>
      <ChangelogList label={labels.added} items={added} />
      <ChangelogList label={labels.changed} items={changed} />
      <ChangelogList label={labels.security} items={security} />
      <ChangelogList label={labels.fixed} items={fixed} />
    </div>
  );
}

function ChangelogList({ label, items }: { label: string; items?: string[] }) {
  if (!items?.length) return null;
  return (
    <div className="changelog-group">
      <span>{label}</span>
      <ul>
        {items.map((item) => <li key={item}>{item}</li>)}
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
      const result = await updateServerConfig({ port: parsed });
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

function GuestSettings({ service, onDone }: { service?: ServiceStatus; onDone: () => Promise<void> }) {
  const { t } = useI18n();
  const [enabled, setEnabled] = useState(Boolean(service?.guest_enabled));
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [recordsOpen, setRecordsOpen] = useState(false);

  useEffect(() => {
    setEnabled(Boolean(service?.guest_enabled));
  }, [service?.guest_enabled]);

  async function toggle(next: boolean) {
    setEnabled(next);
    setMessage('');
    setBusy(true);
    try {
      await updateGuest(next);
      setMessage(next ? t('访客功能已开启') : t('访客功能已关闭'));
      await onDone();
    } catch (err) {
      setEnabled(!next);
      setMessage(err instanceof Error ? err.message : 'guest update failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <article className="panel setting-operation" data-testid="settings-guest">
      <div className="operation-head">
        <div className="operation-icon"><Activity size={18} /></div>
        <div>
          <div className="operation-title-row">
            <h2>{t('访客功能')}</h2>
            <span className={`pill ${enabled ? 'good' : 'warn'}`}>{enabled ? t('已开启') : t('已关闭')}</span>
          </div>
          <p>{enabled ? t('登录页显示访客入口，仅开放脱敏总览') : t('关闭后访客入口和访客总览不可访问')}</p>
        </div>
      </div>
      <div className="settings-button-row">
        <label className="switch-row">
          <input type="checkbox" checked={enabled} disabled={busy} onChange={(event) => toggle(event.target.checked)} />
          <span>{t('允许访客访问')}</span>
        </label>
        <button className="secondary" type="button" onClick={() => setRecordsOpen(true)}>
          <BookOpenText size={16} />
          {t('访客记录')}
        </button>
      </div>
      {message && <p className={message.includes('已') || message.includes('enabled') || message.includes('disabled') ? 'notice' : 'error'}>{message}</p>}
      {recordsOpen && <GuestRecordsDialog onClose={() => setRecordsOpen(false)} />}
    </article>
  );
}

function GuestRecordsDialog({ onClose }: { onClose: () => void }) {
  const { t } = useI18n();
  const visits = useQuery({
    queryKey: ['guest-visits'],
    queryFn: getGuestVisits
  });
  const rows = visits.data?.visits ?? [];

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);

  return createPortal(
    <div className="modal-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <section className="confirm-dialog guest-records-dialog" role="dialog" aria-modal="true" aria-labelledby="guest-records-title" data-testid="guest-records-dialog">
        <div className="panel-head">
          <div>
            <h2 id="guest-records-title">{t('访客记录')}</h2>
            <p>{t('记录最近 100 次访客总览访问')}</p>
          </div>
          <button className="icon-button" type="button" onClick={onClose} title={t('关闭')}>×</button>
        </div>
        <div className="guest-records-list">
          {visits.isLoading && <p className="empty">{t('加载中')}</p>}
          {visits.error instanceof Error && <p className="error">{visits.error.message}</p>}
          {!visits.isLoading && rows.length === 0 && <p className="empty">{t('暂无访客记录')}</p>}
          {rows.map((visit) => (
            <div className="guest-record-row" key={`${visit.at}-${visit.remote_ip}-${visit.fingerprint}`}>
              <div>
                <strong>{visit.remote_ip || '-'}</strong>
                <p>{fmtDateTime(visit.at)} · {visit.fingerprint || '-'}</p>
              </div>
              <div className="guest-record-meta">
                <span>{visit.platform || '-'}</span>
                <span>{visit.language || '-'}</span>
                <span>{visit.screen || '-'}</span>
                <span>{visit.timezone || '-'}</span>
              </div>
              <p className="guest-user-agent">{visit.user_agent || '-'}</p>
            </div>
          ))}
        </div>
      </section>
    </div>,
    document.body
  );
}

function RestartSettings({ service }: { service?: ServiceStatus }) {
  const { t } = useI18n();
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [waiting, setWaiting] = useState(false);
  const [message, setMessage] = useState('');

  async function restart() {
    setConfirmOpen(false);
    setMessage('');
    setBusy(true);
    try {
      const result = await restartService();
      const pending = {
        kind: 'restart',
        restart_at: result.restart_at,
        started_at: new Date().toISOString()
      } satisfies PendingUpdateNotice;
      storePendingUpdate(pending);
      setWaiting(true);
      setMessage(t('服务端正在重启，恢复后页面会自动刷新。'));
      void waitForServerAfterRestart(pending);
    } catch (err) {
      setWaiting(false);
      setMessage(err instanceof Error ? err.message : 'service restart failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <article className="panel setting-operation" data-testid="settings-restart">
      <div className="operation-head">
        <div className="operation-icon"><RefreshCw size={18} /></div>
        <div>
          <h2>{t('重启服务')}</h2>
          <p>{service ? `${service.current_addr} · ${service.current_scheme.toUpperCase()}` : t('等待服务端配置')}</p>
        </div>
        {service?.restart_required && <span className="pill warn">{t('需要重启')}</span>}
      </div>
      <button className="secondary action-button" type="button" onClick={() => setConfirmOpen(true)} disabled={busy || waiting}>
        <RefreshCw size={16} />
        {waiting ? t('重启中') : t('重启服务')}
      </button>
      {confirmOpen && <RestartConfirmDialog busy={busy} onCancel={() => setConfirmOpen(false)} onConfirm={restart} />}
      {waiting && <RestartProgress />}
      {message && <p className={message.includes('正在') || message.includes('restarting') ? 'notice' : 'error'}>{message}</p>}
    </article>
  );
}

function RestartConfirmDialog({ busy, onCancel, onConfirm }: { busy: boolean; onCancel: () => void; onConfirm: () => void }) {
  const { t } = useI18n();
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onCancel();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onCancel]);

  return createPortal(
    <div className="modal-backdrop" role="presentation" onMouseDown={(event) => {
      if (event.target === event.currentTarget) onCancel();
    }}>
      <section className="confirm-dialog warning" role="dialog" aria-modal="true" aria-labelledby="restart-confirm-title" data-testid="restart-confirm-dialog">
        <div className="confirm-icon"><RefreshCw size={22} /></div>
        <div className="confirm-copy">
          <span>{t('重启服务')}</span>
          <h2 id="restart-confirm-title">{t('确认重启服务端？')}</h2>
          <p>{t('服务端会立即调度重启，页面将全屏等待服务恢复，恢复后自动刷新并提示重启成功。')}</p>
        </div>
        <div className="confirm-actions">
          <button className="secondary" type="button" onClick={onCancel} disabled={busy}>{t('取消')}</button>
          <button className="primary compact" type="button" onClick={onConfirm} disabled={busy}>
            <RefreshCw size={16} />
            {busy ? t('重启中') : t('确认重启')}
          </button>
        </div>
      </section>
    </div>,
    document.body
  );
}

function RestartProgress() {
  const { t } = useI18n();
  const stages = [
    t('已发送重启请求'),
    t('服务端正在停止当前进程'),
    t('等待服务端恢复，恢复后自动刷新')
  ];
  return createPortal(
    <div className="modal-backdrop update-progress-backdrop" role="presentation" data-testid="restart-progress">
      <section className="confirm-dialog update-progress-dialog" role="status" aria-live="polite">
        <div className="update-progress-head">
          <div className="confirm-icon"><RefreshCw size={18} /></div>
          <div className="confirm-copy">
            <span>{t('重启服务')}</span>
            <h2>{t('正在重启服务端')}</h2>
            <p>{t('页面正在等待服务恢复')}</p>
          </div>
          <strong>99%</strong>
        </div>
        <div className="update-progress-bar"><span style={{ width: '99%' }} /></div>
        <div className="update-progress">
          {stages.map((label, index) => (
            <div className={index < 2 ? 'done' : 'active'} key={label}>
              <span>{index + 1}</span>
              <p>{label}</p>
            </div>
          ))}
        </div>
      </section>
    </div>,
    document.body
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
  const { t } = useI18n();
  const size = fmtBytes(data?.database_size_bytes ?? 0);
  const days = storedDaysValue(data?.metric_stored_days, data?.retention_hours);
  const free = fmtBytes(data?.disk.free_bytes);
  return (
    <article className="panel setting-operation" data-testid="settings-database">
      <div className="operation-head">
        <div className="operation-icon"><Database size={18} /></div>
        <div>
          <h2>{t('数据库下载')}</h2>
          <p>{t('数据库大小 {size} · 已存储 {days} 天 · {free} 空闲', { size, days, free })}</p>
        </div>
      </div>
      <a className="secondary action-button" href={databaseDownloadURL()} download>
        <Download size={16} />
        {t('下载数据库')}
      </a>
    </article>
  );
}

function DiskReserveSettings({ data, onDone }: { data?: Overview; onDone: () => Promise<void> }) {
  const { t } = useI18n();
  const currentBytes = data?.service.min_free_bytes ?? data?.min_free_space_bytes ?? data?.disk.min_free_bytes ?? 800 * 1024 * 1024;
  const [minFreeMB, setMinFreeMB] = useState(String(Math.round(currentBytes / 1024 / 1024)));
  const [message, setMessage] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setMinFreeMB(String(Math.round(currentBytes / 1024 / 1024)));
  }, [currentBytes]);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setMessage('');
    const parsed = Number(minFreeMB);
    if (!Number.isInteger(parsed) || parsed < 64) {
      setMessage(t('磁盘预留至少 64 MiB'));
      return;
    }
    setBusy(true);
    try {
      await updateServerConfig({ min_free_mb: parsed });
      setMessage(t('磁盘预留已保存'));
      await onDone();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : 'disk reserve update failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <article className="panel setting-operation" data-testid="settings-disk-reserve">
      <div className="operation-head">
        <div className="operation-icon"><Database size={18} /></div>
        <div>
          <h2>{t('磁盘预留')}</h2>
          <p>{fmtBytes(currentBytes)} · {t('空闲 {value}', { value: fmtBytes(data?.disk.free_bytes) })}</p>
        </div>
      </div>
      <form className="settings-form inline" onSubmit={submit}>
        <label>{t('预留空间 MiB')}<input value={minFreeMB} onChange={(event) => setMinFreeMB(event.target.value)} type="number" min={64} step={64} inputMode="numeric" /></label>
        <button className="primary compact" disabled={busy}><Save size={16} />{busy ? t('保存中') : t('保存预留')}</button>
      </form>
      {message && <p className={message.includes('已') || message.includes('saved') ? 'notice' : 'error'}>{message}</p>}
    </article>
  );
}

function UtilChart({ items, theme, compact = false }: { items: StoredGPU[]; theme: Theme; compact?: boolean }) {
  const axisColor = theme === 'dark' ? '#9aa8b5' : '#697789';
  const barColor = theme === 'dark' ? '#4db6ac' : '#146c78';
  const option = useMemo(() => ({
    tooltip: {},
    animation: true,
    animationDuration: 260,
    animationDurationUpdate: 360,
    animationEasingUpdate: 'cubicOut' as const,
    grid: { left: 32, right: 12, top: compact ? 12 : 22, bottom: compact ? 18 : 24 },
    xAxis: { type: 'category', data: items.map((item) => item.gpu.gpu_id), axisLabel: { color: axisColor } },
    yAxis: { type: 'value', max: 100, axisLabel: { color: axisColor }, splitLine: { lineStyle: { color: theme === 'dark' ? '#2c3741' : '#d9e0e7' } } },
    series: [{ type: 'bar', id: 'gpu-utilization', animationDuration: 0, animationDurationUpdate: 360, data: items.map((item) => item.gpu.utilization_gpu_percent ?? 0), itemStyle: { color: barColor, borderRadius: [4, 4, 0, 0] } }]
  }), [axisColor, barColor, compact, items, theme]);

  return <EChart option={option} />;
}

function EChart({ option }: { option: echarts.EChartsCoreOption }) {
  const ref = React.useRef<HTMLDivElement>(null);
  const chartRef = React.useRef<echarts.ECharts | null>(null);
  React.useEffect(() => {
    if (!ref.current) return;
    const chart = echarts.init(ref.current);
    chartRef.current = chart;
    const resize = () => chart.resize();
    window.addEventListener('resize', resize);
    return () => {
      window.removeEventListener('resize', resize);
      chart.dispose();
      chartRef.current = null;
    };
  }, []);
  React.useEffect(() => {
    chartRef.current?.setOption(option, { notMerge: false, lazyUpdate: true });
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
