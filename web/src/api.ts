export type DiskStatus = {
  free_bytes: number;
  min_free_bytes: number;
  status: 'ok' | 'warning' | 'critical';
};

export type GPUStatus = {
  gpu_id: string;
  uuid_hash: string;
  name: string;
  driver_version: string;
  memory_total_bytes: number;
  memory_used_bytes: number;
  utilization_gpu_percent?: number;
  temperature_celsius?: number;
  power_draw_watts?: number;
  fan_speed_percent?: number;
  graphics_clock_mhz?: number;
  memory_clock_mhz?: number;
  pstate?: string;
  pcie_link_generation?: string;
  pcie_link_width?: string;
};

export type StoredGPU = {
  device_id: string;
  timestamp: string;
  gpu: GPUStatus;
};

export type Device = {
  id: string;
  alias: string;
  enabled: boolean;
  status?: 'online' | 'offline';
  hostname?: string;
  os?: string;
  os_version?: string;
  agent_version?: string;
  gpu_count: number;
  last_seen_at?: string;
  last_sample_at?: string;
  last_error?: string;
};

export type StoredProcess = {
  device_id: string;
  timestamp: string;
  process: {
    gpu_id: string;
    uuid_hash: string;
    pid: number;
    process_name: string;
    used_memory_bytes: number;
  };
};

export type GPUStats = {
  device_id: string;
  gpu_id: string;
  gpu_name: string;
  sample_count: number;
  average_utilization_percent?: number;
  peak_utilization_percent?: number;
  idle_sample_percent: number;
  peak_memory_used_bytes: number;
  memory_total_bytes: number;
  peak_temperature_celsius?: number;
  peak_power_draw_watts?: number;
};

export type Overview = {
  server_time: string;
  device_count: number;
  online_device_count: number;
  gpu_count: number;
  average_utilization: number;
  memory_used_bytes: number;
  memory_total_bytes: number;
  hot_gpu_count: number;
  disk: DiskStatus;
  devices: Device[];
  latest_gpus: StoredGPU[];
  latest_processes: StoredProcess[];
};

export type StatsResponse = {
  hours: number;
  stats: GPUStats[];
};

export type DeviceSecretResponse = {
  device: Device;
  secret: string;
};

export type DeviceResponse = {
  device: Device;
};

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    credentials: 'same-origin',
    headers: {
      'Content-Type': 'application/json',
      ...(options.headers ?? {})
    },
    ...options
  });
  if (!response.ok) {
    const body = await response.json().catch(() => ({}));
    throw new Error(body.error ?? response.statusText);
  }
  return response.json() as Promise<T>;
}

export function login(username: string, password: string) {
  return request<{ ok: boolean }>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ username, password })
  });
}

export function logout() {
  return request<{ ok: boolean }>('/api/v1/auth/logout', { method: 'POST' });
}

export function getOverview() {
  return request<Overview>('/api/v1/overview');
}

export function getStats(hours = 24) {
  return request<StatsResponse>(`/api/v1/stats/gpu-utilization?hours=${hours}`);
}

export function createDevice(alias: string) {
  return request<DeviceSecretResponse>('/api/v1/admin/devices', {
    method: 'POST',
    body: JSON.stringify({ alias })
  });
}

export function setDeviceEnabled(deviceId: string, enabled: boolean) {
  return request<DeviceResponse>(`/api/v1/admin/devices/${encodeURIComponent(deviceId)}/${enabled ? 'enable' : 'disable'}`, {
    method: 'POST'
  });
}

export function rotateDeviceSecret(deviceId: string) {
  return request<DeviceSecretResponse>(`/api/v1/admin/devices/${encodeURIComponent(deviceId)}/rotate-secret`, {
    method: 'POST'
  });
}
