const activeWindowSeconds = 30 * 24 * 60 * 60;
const badgeCacheSeconds = 60 * 60;
const maxReportBytes = 8192;
const maxReportClockSkewSeconds = 48 * 60 * 60;
const minReportIntervalSeconds = 5 * 60;

export default {
  async fetch(request, env) {
    try {
      const url = new URL(request.url);
      if (request.method === 'OPTIONS') return noContent();
      if (request.method === 'POST' && url.pathname === '/v1/report') {
        return await handleReport(request, env);
      }
      if (request.method === 'GET' && url.pathname === '/badge') {
        return await handleBadge(env);
      }
      if (request.method === 'GET' && (url.pathname === '/' || url.pathname === '/summary')) {
        return jsonResponse(await loadSummary(env));
      }
      return jsonResponse({ error: 'not found' }, 404);
    } catch (err) {
      if (err instanceof ResponseError) {
        return jsonResponse({ error: err.message }, err.status);
      }
      throw err;
    }
  }
};

async function handleReport(request, env) {
  enforceReportRequest(request);
  const report = validateReport(await parseJsonBody(request));
  const nowEpoch = Math.floor(Date.now() / 1000);
  const reportedAtEpoch = Math.floor(Date.parse(report.reported_at) / 1000) || nowEpoch;
  const ageSeconds = Math.abs(nowEpoch - reportedAtEpoch);
  if (ageSeconds > maxReportClockSkewSeconds) {
    throw badRequest('reported_at outside accepted clock skew');
  }
  const existing = await env.DB.prepare(`
    SELECT last_seen_epoch
    FROM installs
    WHERE install_id_hash = ?
  `).bind(report.install_id_hash).first();
  const lastSeenEpoch = Number(existing?.last_seen_epoch ?? 0);
  if (lastSeenEpoch > 0 && nowEpoch - lastSeenEpoch < minReportIntervalSeconds) {
    return jsonResponse({ accepted: true, throttled: true }, 202, {
      'Retry-After': String(minReportIntervalSeconds)
    });
  }
  await env.DB.prepare(`
    INSERT INTO installs (
      install_id_hash,
      version,
      commit_sha,
      server_os,
      server_arch,
      clients_total,
      clients_active_7d,
      gpus_total,
      gpus_active_7d,
      first_seen_epoch,
      last_seen_epoch,
      reported_at_epoch,
      report_count
    )
    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
    ON CONFLICT(install_id_hash) DO UPDATE SET
      version = excluded.version,
      commit_sha = excluded.commit_sha,
      server_os = excluded.server_os,
      server_arch = excluded.server_arch,
      clients_total = excluded.clients_total,
      clients_active_7d = excluded.clients_active_7d,
      gpus_total = excluded.gpus_total,
      gpus_active_7d = excluded.gpus_active_7d,
      last_seen_epoch = excluded.last_seen_epoch,
      reported_at_epoch = excluded.reported_at_epoch,
      report_count = installs.report_count + 1
  `).bind(
    report.install_id_hash,
    report.version,
    cleanText(report.commit ?? '', 80) || null,
    report.server_os,
    report.server_arch,
    report.clients_total,
    report.clients_active_7d,
    report.gpus_total,
    report.gpus_active_7d,
    nowEpoch,
    nowEpoch,
    reportedAtEpoch
  ).run();
  return jsonResponse({ accepted: true }, 202);
}

async function parseJsonBody(request) {
  const raw = await request.text();
  if (new TextEncoder().encode(raw).byteLength > maxReportBytes) {
    throw new ResponseError('report body too large', 413);
  }
  try {
    return JSON.parse(raw);
  } catch {
    return undefined;
  }
}

function enforceReportRequest(request) {
  const length = Number(request.headers.get('content-length') ?? '0');
  if (Number.isFinite(length) && length > maxReportBytes) {
    throw new ResponseError('report body too large', 413);
  }
  const contentType = request.headers.get('content-type') ?? '';
  if (!contentType.toLowerCase().includes('application/json')) {
    throw badRequest('invalid content type');
  }
  const userAgent = cleanText(request.headers.get('user-agent') ?? '', 120);
  if (!/^GPUFleet\/[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$/.test(userAgent)) {
    throw new ResponseError('invalid telemetry client', 403);
  }
}

async function handleBadge(env) {
  const summary = await loadSummary(env);
  return jsonResponse({
    schemaVersion: 1,
    label: 'GPUFleet GPUs',
    message: formatCount(summary.gpus_active_30d),
    color: '146c78',
    cacheSeconds: badgeCacheSeconds
  }, 200, {
    'Cache-Control': `public, max-age=${badgeCacheSeconds}`
  });
}

async function loadSummary(env) {
  const minEpoch = Math.floor(Date.now() / 1000) - activeWindowSeconds;
  const row = await env.DB.prepare(`
    SELECT
      COUNT(*) AS servers_active_30d,
      COALESCE(SUM(clients_active_7d), 0) AS agents_active_30d,
      COALESCE(SUM(gpus_active_7d), 0) AS gpus_active_30d,
      MAX(last_seen_epoch) AS last_seen_epoch
    FROM installs
    WHERE last_seen_epoch >= ?
  `).bind(minEpoch).first();
  const lastSeen = row?.last_seen_epoch ? new Date(row.last_seen_epoch * 1000).toISOString() : null;
  return {
    servers_active_30d: Number(row?.servers_active_30d ?? 0),
    agents_active_30d: Number(row?.agents_active_30d ?? 0),
    gpus_active_30d: Number(row?.gpus_active_30d ?? 0),
    last_seen_at: lastSeen
  };
}

function validateReport(value) {
  if (!value || typeof value !== 'object') throw badRequest('invalid JSON body');
  if (value.schema_version !== 1) throw badRequest('unsupported schema version');
  const installIDHash = cleanText(value.install_id_hash, 80);
  if (!/^sha256:[a-f0-9]{64}$/.test(installIDHash)) throw badRequest('invalid install id');
  const version = cleanText(value.version, 40);
  const serverOS = cleanText(value.server_os, 40);
  const serverArch = cleanText(value.server_arch, 40);
  const reportedAt = cleanText(value.reported_at, 40);
  if (!version || !serverOS || !serverArch || Number.isNaN(Date.parse(reportedAt))) {
    throw badRequest('missing required fields');
  }
  const clientsTotal = integer(value.clients_total, 0, 10000);
  const clientsActive = integer(value.clients_active_7d, 0, clientsTotal);
  const gpusTotal = integer(value.gpus_total, 0, 100000);
  const gpusActive = integer(value.gpus_active_7d, 0, gpusTotal);
  return {
    schema_version: 1,
    install_id_hash: installIDHash,
    version,
    commit: cleanText(value.commit, 80),
    server_os: serverOS,
    server_arch: serverArch,
    clients_total: clientsTotal,
    clients_active_7d: clientsActive,
    gpus_total: gpusTotal,
    gpus_active_7d: gpusActive,
    reported_at: reportedAt
  };
}

function integer(value, min, max) {
  if (typeof value !== 'number' || !Number.isFinite(value)) throw badRequest('invalid numeric field');
  const rounded = Math.floor(value);
  if (rounded < min || rounded > max) throw badRequest('numeric field out of range');
  return rounded;
}

function cleanText(value, maxLength) {
  if (typeof value !== 'string') return '';
  return value.trim().slice(0, maxLength);
}

function formatCount(value) {
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(value);
}

function badRequest(message) {
  return new ResponseError(message, 400);
}

class ResponseError extends Error {
  constructor(message, status) {
    super(message);
    this.status = status;
  }
}

function jsonResponse(value, status = 200, headers = {}) {
  return new Response(JSON.stringify(value), {
    status,
    headers: {
      'Content-Type': 'application/json; charset=utf-8',
      'X-Robots-Tag': 'noindex',
      ...headers
    }
  });
}

function noContent() {
  return new Response(null, { status: 204 });
}
