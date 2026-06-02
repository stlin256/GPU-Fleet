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
  vbios_version?: string;
  memory_total_bytes: number;
  memory_used_bytes: number;
  memory_free_bytes?: number;
  memory_reserved_bytes?: number;
  utilization_gpu_percent?: number;
  utilization_memory_percent?: number;
  temperature_celsius?: number;
  temperature_memory_celsius?: number;
  temperature_limit_celsius?: number;
  power_draw_watts?: number;
  power_limit_watts?: number;
  power_enforced_limit_watts?: number;
  fan_speed_percent?: number;
  graphics_clock_mhz?: number;
  memory_clock_mhz?: number;
  sm_clock_mhz?: number;
  video_clock_mhz?: number;
  pstate?: string;
  pcie_link_generation?: string;
  pcie_link_width?: string;
  pcie_link_generation_max?: string;
  pcie_link_width_max?: string;
  compute_mode?: string;
  compute_capability?: string;
  display_active?: string;
  display_attached?: string;
  persistence_mode?: string;
  driver_model?: string;
  ecc_mode_current?: string;
  mig_mode_current?: string;
  clock_throttle_reasons?: string;
  collection_error?: string;
};

export type StoredGPU = {
  device_id: string;
  timestamp: string;
  gpu: GPUStatus;
};

export type GPUSeriesPoint = {
  timestamp: string;
  utilization_gpu_percent?: number;
  memory_used_bytes: number;
  memory_total_bytes: number;
  temperature_celsius?: number;
  power_draw_watts?: number;
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

export type ServiceStatus = {
  current_addr: string;
  current_scheme: 'http' | 'https';
  configured_addr: string;
  configured_port: number;
  https_enabled: boolean;
  cert_not_after?: string;
  config_revision: number;
  updated_at?: string;
  restart_required: boolean;
  first_startup_http: boolean;
  management_base_url?: string;
};

export type SetupStatus = {
  setup_required: boolean;
  setup_complete: boolean;
  service: ServiceStatus;
};

export type SetupPayload = {
  password?: string;
  port?: number;
  certificate_pem?: string;
  private_key_pem?: string;
};

export type ServiceMutationResponse = {
  ok: boolean;
  service: ServiceStatus;
  restart_required: boolean;
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
  retention_hours: number;
  min_free_space_bytes: number;
  setup_complete: boolean;
  service: ServiceStatus;
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

export function getSetupStatus() {
  return request<SetupStatus>('/api/v1/setup/status');
}

export function applyInitialSetup(payload: SetupPayload) {
  return request<ServiceMutationResponse>('/api/v1/setup/apply', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}

export function applySetup(payload: SetupPayload) {
  return request<ServiceMutationResponse>('/api/v1/admin/setup/apply', {
    method: 'POST',
    body: JSON.stringify(payload)
  });
}

export function reopenSetup() {
  return request<{ ok: boolean; setup: SetupStatus }>('/api/v1/admin/setup/reopen', {
    method: 'POST'
  });
}

export function login(password: string) {
  return request<{ ok: boolean }>('/api/v1/auth/login', {
    method: 'POST',
    body: JSON.stringify({ password })
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

export function getGPUSeries(deviceId: string, gpuId: string, hours = 1) {
  return request<GPUSeriesPoint[]>(`/api/v1/gpus/${encodeURIComponent(gpuId)}/series?device_id=${encodeURIComponent(deviceId)}&hours=${hours}`);
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

export function changePassword(currentPassword: string, nextPassword: string) {
  return request<{ ok: boolean }>('/api/v1/admin/password', {
    method: 'POST',
    body: JSON.stringify({ current_password: currentPassword, next_password: nextPassword })
  });
}

export function updateServerConfig(port: number) {
  return request<ServiceMutationResponse>('/api/v1/admin/server-config', {
    method: 'POST',
    body: JSON.stringify({ port })
  });
}

export function uploadCertificate(certificatePEM: string, privateKeyPEM: string) {
  return request<ServiceMutationResponse>('/api/v1/admin/certificate', {
    method: 'POST',
    body: JSON.stringify({ certificate_pem: certificatePEM, private_key_pem: privateKeyPEM })
  });
}

export function databaseDownloadURL() {
  return '/api/v1/admin/database/download';
}
