import assert from 'node:assert/strict';
import test from 'node:test';

import worker from '../src/index.js';

const installIDHash = `sha256:${'a'.repeat(64)}`;

test('accepts valid GPUFleet reports and updates the summary', async () => {
  const env = { DB: new MockDB() };
  const response = await worker.fetch(reportRequest(validReport()), env);

  assert.equal(response.status, 202);
  assert.deepEqual(await response.json(), { accepted: true });
  assert.equal(env.DB.writeCount, 1);

  const summary = await worker.fetch(new Request('https://telemetry.test/summary'), env);
  assert.equal(summary.status, 200);
  assert.deepEqual(await summary.json(), {
    servers_active_30d: 1,
    agents_active_30d: 2,
    gpus_active_30d: 4,
    last_seen_at: new Date(env.DB.rows.get(installIDHash).last_seen_epoch * 1000).toISOString()
  });
});

test('throttles repeated reports from the same install id', async () => {
  const env = { DB: new MockDB() };

  const first = await worker.fetch(reportRequest(validReport()), env);
  assert.equal(first.status, 202);

  const second = await worker.fetch(reportRequest(validReport()), env);
  assert.equal(second.status, 202);
  assert.equal(second.headers.get('Retry-After'), '300');
  assert.deepEqual(await second.json(), { accepted: true, throttled: true });
  assert.equal(env.DB.writeCount, 1);
});

test('rejects oversized reports before parsing the body', async () => {
  const env = { DB: new MockDB() };
  const request = reportRequest(validReport(), {
    headers: { 'Content-Length': '8193' }
  });

  const response = await worker.fetch(request, env);

  assert.equal(response.status, 413);
  assert.equal((await response.json()).error, 'report body too large');
  assert.equal(env.DB.writeCount, 0);
});

test('rejects reports that are not from GPUFleet clients', async () => {
  const env = { DB: new MockDB() };
  const response = await worker.fetch(reportRequest(validReport(), {
    headers: { 'User-Agent': 'curl/8.0.0' }
  }), env);

  assert.equal(response.status, 403);
  assert.equal((await response.json()).error, 'invalid telemetry client');
  assert.equal(env.DB.writeCount, 0);
});

test('rejects reports with abnormal reported_at timestamps', async () => {
  const env = { DB: new MockDB() };
  const oldReport = validReport({
    reported_at: new Date(Date.now() - 72 * 60 * 60 * 1000).toISOString()
  });

  const response = await worker.fetch(reportRequest(oldReport), env);

  assert.equal(response.status, 400);
  assert.equal((await response.json()).error, 'reported_at outside accepted clock skew');
  assert.equal(env.DB.writeCount, 0);
});

function validReport(overrides = {}) {
  return {
    schema_version: 1,
    install_id_hash: installIDHash,
    version: '1.0.11',
    commit: 'abcdef123456',
    server_os: 'linux',
    server_arch: 'amd64',
    clients_total: 3,
    clients_active_7d: 2,
    gpus_total: 5,
    gpus_active_7d: 4,
    reported_at: new Date().toISOString(),
    ...overrides
  };
}

function reportRequest(body, options = {}) {
  const headers = new Headers({
    'Content-Type': 'application/json',
    'User-Agent': 'GPUFleet/1.0.11',
    ...options.headers
  });
  return new Request('https://telemetry.test/v1/report', {
    method: 'POST',
    headers,
    body: JSON.stringify(body)
  });
}

class MockDB {
  constructor() {
    this.rows = new Map();
    this.writeCount = 0;
  }

  prepare(sql) {
    return new MockStatement(this, sql);
  }
}

class MockStatement {
  constructor(db, sql) {
    this.db = db;
    this.sql = sql;
  }

  bind(...args) {
    this.args = args;
    return this;
  }

  async first() {
    if (this.sql.includes('SELECT last_seen_epoch') && this.sql.includes('WHERE install_id_hash')) {
      const row = this.db.rows.get(this.args[0]);
      return row ? { last_seen_epoch: row.last_seen_epoch } : null;
    }
    if (this.sql.includes('COUNT(*) AS servers_active_30d')) {
      const minEpoch = this.args[0];
      const rows = [...this.db.rows.values()].filter((row) => row.last_seen_epoch >= minEpoch);
      return {
        servers_active_30d: rows.length,
        agents_active_30d: rows.reduce((sum, row) => sum + row.clients_active_7d, 0),
        gpus_active_30d: rows.reduce((sum, row) => sum + row.gpus_active_7d, 0),
        last_seen_epoch: rows.reduce((max, row) => Math.max(max, row.last_seen_epoch), 0)
      };
    }
    throw new Error(`unexpected first query: ${this.sql}`);
  }

  async run() {
    if (!this.sql.includes('INSERT INTO installs')) {
      throw new Error(`unexpected run query: ${this.sql}`);
    }
    const existing = this.db.rows.get(this.args[0]);
    this.db.rows.set(this.args[0], {
      install_id_hash: this.args[0],
      version: this.args[1],
      commit_sha: this.args[2],
      server_os: this.args[3],
      server_arch: this.args[4],
      clients_total: this.args[5],
      clients_active_7d: this.args[6],
      gpus_total: this.args[7],
      gpus_active_7d: this.args[8],
      first_seen_epoch: existing?.first_seen_epoch ?? this.args[9],
      last_seen_epoch: this.args[10],
      reported_at_epoch: this.args[11],
      report_count: (existing?.report_count ?? 0) + 1
    });
    this.db.writeCount += 1;
    return { success: true };
  }
}
