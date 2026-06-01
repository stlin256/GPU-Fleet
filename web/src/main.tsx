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
  Gauge,
  HardDrive,
  KeyRound,
  LogIn,
  LogOut,
  MonitorUp,
  Plus,
  Power,
  PowerOff,
  RefreshCw,
  Server,
  Settings,
  ShieldCheck,
  Thermometer,
  Zap
} from 'lucide-react';
import {
  createDevice,
  getOverview,
  getStats,
  GPUStats,
  login,
  logout,
  Overview,
  rotateDeviceSecret,
  setDeviceEnabled,
  StoredGPU,
  StoredProcess
} from './api';
import './styles.css';

echarts.use([BarChart, LineChart, GridComponent, TooltipComponent, CanvasRenderer]);

const queryClient = new QueryClient();
type View = 'overview' | 'devices' | 'gpus' | 'settings';

function fmtBytes(value?: number) {
  if (!value) return '0 B';
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

function App() {
  const [authenticated, setAuthenticated] = useState(false);
  if (!authenticated) {
    return <Login onSuccess={() => setAuthenticated(true)} />;
  }
  return <Dashboard onUnauthorized={() => setAuthenticated(false)} />;
}

function Login({ onSuccess }: { onSuccess: () => void }) {
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setLoading(true);
    setError('');
    try {
      await login(username, password);
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
        <div className="brand">
          <span className="brand-mark">G</span>
          <span>GPUFleet</span>
        </div>
        <h1>登录面板</h1>
        <label>
          用户名
          <input value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" />
        </label>
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

function Dashboard({ onUnauthorized }: { onUnauthorized: () => void }) {
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

        {(view === 'overview' || view === 'gpus') && (
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
                  <h2>GPU 状态</h2>
                  <span>{data?.latest_gpus.length ?? 0}</span>
                </div>
                <div className="gpu-grid">
                  {(data?.latest_gpus ?? []).map((item) => <GPUCard key={`${item.device_id}-${item.gpu.gpu_id}`} item={item} />)}
                </div>
                <UtilChart items={data?.latest_gpus ?? []} />
              </div>
              <div className="stack">
                <DevicePanel data={data} />
                <ProcessPanel items={data?.latest_processes ?? []} />
              </div>
            </section>

            <StatsPanel statRows={statRows} />
          </>
        )}

        {view === 'devices' && <DeviceAdminPanel data={data} />}
        {view === 'settings' && <SettingsPanel data={data} />}
      </main>
    </div>
  );
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
  const mem = gpu.memory_total_bytes ? (gpu.memory_used_bytes / gpu.memory_total_bytes) * 100 : 0;
  return (
    <article className="gpu-card">
      <div className="card-title">
        <div>
          <h3>{gpu.name || gpu.gpu_id}</h3>
          <p>{item.device_id} · {gpu.gpu_id}</p>
        </div>
        <span>{pct(gpu.utilization_gpu_percent)}</span>
      </div>
      <div className="meter"><i style={{ width: `${gpu.utilization_gpu_percent ?? 0}%` }} /></div>
      <div className="kv"><span>显存</span><strong>{pct(mem)} · {fmtBytes(gpu.memory_used_bytes)}</strong></div>
      <div className="kv"><span><Thermometer size={14} />温度</span><strong>{gpu.temperature_celsius ? `${Math.round(gpu.temperature_celsius)}°C` : '-'}</strong></div>
      <div className="kv"><span><Zap size={14} />功耗</span><strong>{gpu.power_draw_watts ? `${gpu.power_draw_watts.toFixed(1)} W` : '-'}</strong></div>
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

function SettingsPanel({ data }: { data?: Overview }) {
  const free = data?.disk.free_bytes ?? 0;
  const min = data?.disk.min_free_bytes ?? 0;
  return (
    <section className="settings-grid">
      <article className="panel">
        <div className="panel-head">
          <h2>服务端边界</h2>
          <ShieldCheck size={18} />
        </div>
        <div className="settings-list">
          <div><strong>客户端控制</strong><p>未提供命令下发、配置下发或远程执行接口</p></div>
          <div><strong>Agent 接入</strong><p>HMAC、时间戳、nonce、请求体大小限制和基础限流</p></div>
          <div><strong>Web 会话</strong><p>HttpOnly Cookie，会话过期后重新登录</p></div>
        </div>
      </article>
      <article className="panel">
        <div className="panel-head">
          <h2>磁盘保护</h2>
          <span className={`pill ${data?.disk.status ?? 'ok'}`}>{data?.disk.status ?? 'ok'}</span>
        </div>
        <div className="settings-list">
          <div><strong>当前空闲</strong><p>{fmtBytes(free)}</p></div>
          <div><strong>保留阈值</strong><p>{fmtBytes(min)}</p></div>
          <div><strong>回收策略</strong><p>写入指标前清理过期 gzip 分段</p></div>
        </div>
      </article>
    </section>
  );
}

function UtilChart({ items }: { items: StoredGPU[] }) {
  const option = useMemo(() => ({
    tooltip: {},
    grid: { left: 32, right: 12, top: 22, bottom: 24 },
    xAxis: { type: 'category', data: items.map((item) => item.gpu.gpu_id), axisLabel: { color: '#748091' } },
    yAxis: { type: 'value', max: 100, axisLabel: { color: '#748091' } },
    series: [{ type: 'bar', data: items.map((item) => item.gpu.utilization_gpu_percent ?? 0), itemStyle: { color: '#1769aa', borderRadius: [4, 4, 0, 0] } }]
  }), [items]);

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
