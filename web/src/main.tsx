import React, { useMemo, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import * as echarts from 'echarts/core';
import { BarChart, LineChart } from 'echarts/charts';
import { GridComponent, TooltipComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import {
  Activity,
  Cpu,
  Database,
  Gauge,
  HardDrive,
  LogIn,
  MonitorUp,
  RefreshCw,
  Server,
  Settings,
  Thermometer,
  Zap
} from 'lucide-react';
import { getOverview, getStats, login, Overview, StoredGPU, StoredProcess } from './api';
import './styles.css';

echarts.use([BarChart, LineChart, GridComponent, TooltipComponent, CanvasRenderer]);

const queryClient = new QueryClient();

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

  if (overview.error instanceof Error && overview.error.message.includes('login')) {
    onUnauthorized();
  }

  const data = overview.data;
  const statRows = stats.data?.stats ?? [];
  const memoryPct = data?.memory_total_bytes ? (data.memory_used_bytes / data.memory_total_bytes) * 100 : 0;

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">G</span>
          <span>GPUFleet</span>
        </div>
        <nav>
          <button className="active"><Activity size={17} />总览</button>
          <button><Server size={17} />设备</button>
          <button><Cpu size={17} />GPU</button>
          <button><Settings size={17} />设置</button>
        </nav>
      </aside>
      <main className="content">
        <header className="topbar">
          <div>
            <h1>GPU 资源总览</h1>
            <p>{data ? `服务端时间 ${new Date(data.server_time).toLocaleString()}` : '等待服务端数据'}</p>
          </div>
          <button className="icon-button" onClick={() => overview.refetch()} title="刷新">
            <RefreshCw size={18} />
          </button>
        </header>

        {data?.disk.status === 'critical' && <div className="banner danger">磁盘空间低于保护阈值，服务端已拒绝新指标写入。</div>}
        {overview.error && <div className="banner danger">{overview.error.message}</div>}

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
          <span className={`pill ${device.status ?? 'offline'}`}>{device.status ?? 'offline'}</span>
        </div>
      ))}
    </section>
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

