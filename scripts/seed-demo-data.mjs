import crypto from 'node:crypto';
import { gzipSync } from 'node:zlib';
import { mkdir, readFile, rm, writeFile } from 'node:fs/promises';
import path from 'node:path';

const options = parseArgs(process.argv.slice(2));
const dataDir = path.resolve(options['data-dir'] || 'logs/manual-demo/data');
const metadataPath = path.join(dataDir, 'metadata.json');
const metricsDir = path.join(dataDir, 'metrics');

const devices = [
  { id: 'rig-dual', secret: 'demo-dual-secret', alias: 'rig-dual', hostname: 'dual-4090', gpus: ['gpu0', 'gpu1'], online: true },
  { id: 'rig-render', secret: 'demo-render-secret', alias: 'rig-render', hostname: 'render-node', gpus: ['gpu0'], online: true },
  { id: 'rig-train', secret: 'demo-train-secret', alias: 'rig-train', hostname: 'train-box', gpus: ['gpu0'], online: true },
  { id: 'rig-offline', secret: 'demo-offline-secret', alias: 'rig-offline', hostname: 'offline-lab', gpus: ['gpu0'], online: false }
];

const gpuProfiles = {
  'rig-dual/gpu0': { name: 'NVIDIA GeForce RTX 4090', total: 24, utilBase: 82, utilAmp: 15, memBase: 15, tempBase: 71, powerBase: 350, powerLimit: 450 },
  'rig-dual/gpu1': { name: 'NVIDIA GeForce RTX 4090', total: 24, utilBase: 38, utilAmp: 24, memBase: 10, tempBase: 58, powerBase: 210, powerLimit: 450 },
  'rig-render/gpu0': { name: 'NVIDIA RTX 6000 Ada', total: 48, utilBase: 64, utilAmp: 18, memBase: 31, tempBase: 66, powerBase: 245, powerLimit: 300 },
  'rig-train/gpu0': { name: 'NVIDIA A100 80GB PCIe', total: 80, utilBase: 93, utilAmp: 8, memBase: 69, tempBase: 79, powerBase: 285, powerLimit: 300 },
  'rig-offline/gpu0': { name: 'NVIDIA GeForce RTX 3090', total: 24, utilBase: 12, utilAmp: 9, memBase: 5, tempBase: 42, powerBase: 90, powerLimit: 350 }
};

async function main() {
  const raw = JSON.parse(await readFile(metadataPath, 'utf8'));
  if (!raw.admin?.username) throw new Error(`metadata at ${metadataPath} does not contain an admin account`);

  const now = Date.now();
  raw.devices = {};
  for (const device of devices) {
    const seenAt = new Date(device.online ? now + 30 * 60_000 : now - 20 * 60_000).toISOString();
    const sampleAt = new Date(device.online ? now : now - 20 * 60_000).toISOString();
    raw.devices[device.id] = {
      id: device.id,
      alias: device.alias,
      secret: device.secret,
      enabled: true,
      created_at: new Date(now - 2 * 60 * 60_000).toISOString(),
      last_seen_at: seenAt,
      agent_version: '0.1.1-demo',
      hostname: device.hostname,
      os: device.id === 'rig-train' ? 'linux' : 'windows',
      os_version: device.id === 'rig-train' ? 'Ubuntu 24.04' : 'Windows 11',
      gpu_count: device.gpus.length,
      last_sample_at: sampleAt,
      last_error: device.online ? '' : 'demo_offline'
    };
  }
  raw.audit_events = raw.audit_events || [];
  await writeFile(metadataPath, JSON.stringify(raw, null, 2));

  await rm(metricsDir, { recursive: true, force: true });
  await mkdir(metricsDir, { recursive: true });
  const bySegment = new Map();
  for (const device of devices) {
    for (let i = 35; i >= 0; i -= 1) {
      const timestamp = new Date(now - i * 2 * 60_000 - (device.online ? 0 : 20 * 60_000));
      const sample = {
        device_id: device.id,
        agent_version: '0.1.1-demo',
        timestamp: timestamp.toISOString(),
        gpus: device.gpus.map((gpuID) => gpuSample(device, gpuID, i))
      };
      const segment = segmentName(timestamp);
      bySegment.set(segment, [...(bySegment.get(segment) || []), JSON.stringify(sample)]);
    }
  }
  for (const [segment, lines] of bySegment.entries()) {
    await writeFile(path.join(metricsDir, `samples-${segment}.jsonl.gz`), gzipSync(`${lines.join('\n')}\n`));
  }

  console.log(JSON.stringify({
    ok: true,
    data_dir: dataDir,
    devices: devices.length,
    gpus: devices.reduce((sum, device) => sum + device.gpus.length, 0),
    dual_device: 'rig-dual',
    offline_device: 'rig-offline'
  }, null, 2));
}

function gpuSample(device, gpuID, index) {
  const profile = gpuProfiles[`${device.id}/${gpuID}`];
  const phase = index / 4;
  const util = clamp(profile.utilBase + Math.sin(phase) * profile.utilAmp + Math.cos(phase * 0.7) * 6, 1, 100);
  const memoryGiB = clamp(profile.memBase + Math.sin(phase * 0.8) * 2.8 + util / 18, 1, profile.total - 1);
  const temp = clamp(profile.tempBase + util / 14 + Math.sin(phase * 1.3) * 3, 30, 92);
  const power = clamp(profile.powerBase + util * 1.15 + Math.cos(phase) * 12, 30, profile.powerLimit);
  const totalBytes = profile.total * 1024 ** 3;
  const usedBytes = Math.round(memoryGiB * 1024 ** 3);
  const uuid = `${device.id}-${gpuID}-demo`;
  return {
    gpu_id: gpuID,
    uuid_hash: `sha256:${sha256(uuid)}`,
    name: profile.name,
    driver_version: '591.74',
    vbios_version: 'demo',
    memory_total_bytes: totalBytes,
    memory_used_bytes: usedBytes,
    memory_free_bytes: totalBytes - usedBytes,
    utilization_gpu_percent: round(util),
    utilization_memory_percent: round((usedBytes / totalBytes) * 100),
    temperature_celsius: round(temp),
    power_draw_watts: round(power),
    power_limit_watts: profile.powerLimit,
    power_enforced_limit_watts: profile.powerLimit,
    fan_speed_percent: clamp(Math.round(35 + util / 2), 20, 95),
    graphics_clock_mhz: Math.round(1700 + util * 10),
    memory_clock_mhz: 16000,
    sm_clock_mhz: Math.round(1600 + util * 9),
    video_clock_mhz: 1200,
    pstate: util > 50 ? 'P0' : 'P2',
    pcie_link_generation: '4',
    pcie_link_width: gpuID === 'gpu1' ? '8' : '16',
    pcie_link_generation_max: '4',
    pcie_link_width_max: '16',
    compute_mode: 'Default',
    compute_capability: profile.name.includes('A100') ? '8.0' : '8.9',
    display_active: 'Disabled',
    display_attached: 'No',
    driver_model: device.id === 'rig-train' ? 'N/A' : 'WDDM',
    clock_throttle_reasons: '0x0000000000000000'
  };
}

function segmentName(date) {
  const yyyy = date.getUTCFullYear();
  const mm = String(date.getUTCMonth() + 1).padStart(2, '0');
  const dd = String(date.getUTCDate()).padStart(2, '0');
  const hh = String(date.getUTCHours()).padStart(2, '0');
  return `${yyyy}${mm}${dd}${hh}`;
}

function sha256(value) {
  return crypto.createHash('sha256').update(value).digest('hex');
}

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function round(value) {
  return Math.round(value * 10) / 10;
}

function parseArgs(args) {
  const parsed = {};
  for (let i = 0; i < args.length; i += 1) {
    if (!args[i].startsWith('--')) continue;
    parsed[args[i].slice(2)] = args[i + 1];
    i += 1;
  }
  return parsed;
}

main().catch((err) => {
  console.error(err?.stack || err);
  process.exit(1);
});
