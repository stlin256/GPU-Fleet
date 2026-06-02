import { mkdir, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { spawn } from 'node:child_process';
import net from 'node:net';

async function main() {
  const options = parseArgs(process.argv.slice(2));
  const chromePath = options.chrome || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';
  const targetURL = required(options.url, '--url');
  const username = options.username || 'admin';
  const password = required(options.password, '--password');
  const outDir = options.out || path.resolve('logs', `frontend-verify-${Date.now()}`);

  const port = await freePort();
  const profileDir = path.join(tmpdir(), `gpufleet-chrome-${process.pid}-${Date.now()}`);
  await mkdir(outDir, { recursive: true });

  const chrome = spawn(chromePath, [
    '--headless=new',
    `--remote-debugging-port=${port}`,
    '--remote-debugging-address=127.0.0.1',
    '--remote-allow-origins=*',
    `--user-data-dir=${profileDir}`,
    '--disable-extensions',
    '--disable-component-extensions-with-background-pages',
    '--disable-background-networking',
    '--disable-dev-shm-usage',
    '--disable-gpu',
    '--disable-gpu-compositing',
    '--disable-gpu-sandbox',
    '--disable-software-rasterizer',
    '--disable-features=VizDisplayCompositor',
    '--hide-scrollbars',
    '--no-sandbox',
    '--no-default-browser-check',
    '--no-first-run',
    '--window-size=1440,1000',
    'about:blank'
  ], { stdio: ['ignore', 'ignore', 'pipe'] });

  let stderr = '';
  chrome.stderr.on('data', (chunk) => {
    stderr += chunk.toString();
  });

  try {
    logStep('waiting for Chrome CDP');
    await waitForChrome(port, () => stderr);
    logStep('creating tab');
    const tabInfo = await createTab(port, targetURL);
    logStep(`connecting CDP ${tabInfo.webSocketDebuggerUrl}`);
    const cdp = await CDPConnection.open(tabInfo.webSocketDebuggerUrl);
    try {
      logStep('enabling page/runtime domains');
      await cdp.send('Page.enable');
      await cdp.send('Runtime.enable');
      await cdp.send('Emulation.setDeviceMetricsOverride', {
        width: 1440,
        height: 1000,
        deviceScaleFactor: 1,
        mobile: false
      });

      await waitForLoad(cdp);
      await setStoredTheme(cdp, 'light');
      await cdp.send('Page.reload', { ignoreCache: true });
      await waitForLoad(cdp);
      logStep('logging in');
      await login(cdp, username, password);
      await waitForText(cdp, ['GPUFleet', text('GPU resource overview'), text('fleet board'), text('fleet live')], 12000);
      await assertNoConsoleErrors(cdp);
      await assertFleetBoard(cdp);
      await waitForTheme(cdp, 'light');
      logStep('capturing desktop light overview');
      await screenshot(cdp, path.join(outDir, 'desktop-overview.png'));

      logStep('checking dark theme');
      await clickTestId(cdp, 'theme-toggle');
      await waitForTheme(cdp, 'dark');
      await screenshot(cdp, path.join(outDir, 'desktop-overview-dark.png'));

      logStep('checking reload session restore');
      await cdp.send('Page.reload', { ignoreCache: true });
      await waitForLoad(cdp);
      await waitForText(cdp, ['GPUFleet', text('GPU resource overview'), text('fleet board')], 12000);
      const dashboardAfterReload = await visibleText(cdp);
      if (dashboardAfterReload.includes(text('login panel'))) {
        throw new Error('session was not restored after page reload');
      }
      await waitForTheme(cdp, 'dark');

      logStep('checking devices view');
      await clickButton(cdp, text('devices'));
      await waitForText(cdp, [text('device management'), text('register device')], 5000);
      await screenshot(cdp, path.join(outDir, 'desktop-devices.png'));

      logStep('checking mobile overview');
      await clickButton(cdp, text('overview'));
      await cdp.send('Emulation.setDeviceMetricsOverride', {
        width: 390,
        height: 900,
        deviceScaleFactor: 1,
        mobile: true
      });
      await cdp.send('Page.reload', { ignoreCache: true });
      await waitForLoad(cdp);
      await waitForText(cdp, ['GPUFleet', text('GPU resource overview'), text('fleet board')], 12000);
      await screenshot(cdp, path.join(outDir, 'mobile-overview.png'));
      const mobileOverviewLayout = await evaluate(cdp, () => ({
        width: window.innerWidth,
        scrollWidth: document.documentElement.scrollWidth,
        fleetRowCount: document.querySelectorAll('.fleet-row:not(.fleet-header)').length,
        theme: document.documentElement.dataset.theme || '',
        bodyText: document.body.innerText
      }));
      if (mobileOverviewLayout.scrollWidth > mobileOverviewLayout.width + 2) {
        throw new Error(`mobile overview overflows horizontally: ${mobileOverviewLayout.scrollWidth} > ${mobileOverviewLayout.width}`);
      }
      if (mobileOverviewLayout.fleetRowCount < 1) {
        throw new Error('fleet overview rows were not rendered in mobile browser');
      }

      logStep('checking mobile GPU view');
      await clickButton(cdp, text('gpu'));
      await waitForText(cdp, [text('gpu monitoring')], 5000);
      await screenshot(cdp, path.join(outDir, 'mobile-gpu.png'));

      const layout = await evaluate(cdp, () => ({
        width: window.innerWidth,
        scrollWidth: document.documentElement.scrollWidth,
        title: document.querySelector('h1')?.textContent || '',
        cardCount: document.querySelectorAll('.gpu-card').length,
        theme: document.documentElement.dataset.theme || '',
        buttonCount: document.querySelectorAll('button').length,
        bodyText: document.body.innerText
      }));
      if (layout.scrollWidth > layout.width + 2) {
        throw new Error(`mobile layout overflows horizontally: ${layout.scrollWidth} > ${layout.width}`);
      }
      if (layout.cardCount < 1) {
        throw new Error('GPU card was not rendered in browser');
      }
      if (layout.theme !== 'dark') {
        throw new Error(`theme did not persist after reload: ${layout.theme}`);
      }
      if (!layout.bodyText.includes('NVIDIA')) {
        throw new Error('GPU model was not visible in browser');
      }
      if (!layout.bodyText.includes('Compute') || !layout.bodyText.includes('PCIe')) {
        throw new Error('expanded GPU runtime details were not visible in browser');
      }

      const result = {
        ok: true,
        screenshots: {
          desktop_overview: path.join(outDir, 'desktop-overview.png'),
          desktop_overview_dark: path.join(outDir, 'desktop-overview-dark.png'),
          desktop_devices: path.join(outDir, 'desktop-devices.png'),
          mobile_overview: path.join(outDir, 'mobile-overview.png'),
          mobile_gpu: path.join(outDir, 'mobile-gpu.png')
        },
        layout: {
          width: layout.width,
          scrollWidth: layout.scrollWidth,
          cardCount: layout.cardCount,
          fleetRowCount: mobileOverviewLayout.fleetRowCount,
          theme: layout.theme,
          buttonCount: layout.buttonCount
        }
      };
      await writeFile(path.join(outDir, 'result.json'), JSON.stringify(result, null, 2));
      console.log(JSON.stringify(result, null, 2));
    } finally {
      await cdp.close().catch(() => undefined);
    }
  } finally {
    chrome.kill('SIGKILL');
    await waitForExit(chrome).catch(() => undefined);
    await rm(profileDir, { recursive: true, force: true }).catch(() => undefined);
  }
}

function parseArgs(args) {
  const parsed = {};
  for (let i = 0; i < args.length; i += 1) {
    const arg = args[i];
    if (!arg.startsWith('--')) continue;
    const key = arg.slice(2);
    parsed[key] = args[i + 1];
    i += 1;
  }
  return parsed;
}

function required(value, name) {
  if (!value) throw new Error(`${name} is required`);
  return value;
}

function text(id) {
  const values = {
    'GPU resource overview': '\u0047\u0050\u0055 \u8d44\u6e90\u603b\u89c8',
    'fleet board': '\u0047\u0050\u0055 \u0046\u006c\u0065\u0065\u0074',
    'fleet live': '\u591a\u673a \u0047\u0050\u0055 \u8fd0\u884c\u6001',
    'device management': '\u8bbe\u5907\u7ba1\u7406',
    'register device': '\u6ce8\u518c\u8bbe\u5907',
    overview: '\u603b\u89c8',
    devices: '\u8bbe\u5907',
    gpu: '\u0047\u0050\u0055',
    'gpu monitoring': '\u0047\u0050\u0055 \u76d1\u63a7',
    'login panel': '\u767b\u5f55\u9762\u677f'
  };
  return values[id] || id;
}

async function freePort() {
  const server = net.createServer();
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const address = server.address();
  await new Promise((resolve) => server.close(resolve));
  return address.port;
}

async function waitForChrome(port, stderrText) {
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline) {
    try {
      const res = await fetch(`http://127.0.0.1:${port}/json/version`);
      if (res.ok) return;
    } catch {
      await delay(120);
    }
  }
  throw new Error(`Chrome did not expose CDP on port ${port}. ${stderrText().slice(0, 500)}`);
}

async function createTab(port, url) {
  const res = await fetch(`http://127.0.0.1:${port}/json/new?${encodeURIComponent(url)}`, { method: 'PUT' });
  if (!res.ok) throw new Error(`create tab failed: ${res.status} ${await res.text()}`);
  return res.json();
}

async function login(cdp, user, pass) {
  await waitForText(cdp, [text('login panel')], 10000);
  await evaluate(cdp, ({ user, pass }) => {
    const setValue = (input, value) => {
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set;
      setter.call(input, value);
      input.dispatchEvent(new Event('input', { bubbles: true }));
    };
    const inputs = Array.from(document.querySelectorAll('input'));
    if (inputs.length < 2) throw new Error('login inputs not found');
    setValue(inputs[0], user);
    setValue(inputs[1], pass);
    document.querySelector('form').requestSubmit();
  }, { user, pass });
}

async function clickButton(cdp, label) {
  await evaluate(cdp, (label) => {
    const button = Array.from(document.querySelectorAll('button')).find((item) =>
      (item.textContent || '').includes(label)
    );
    if (!button) throw new Error(`button not found: ${label}`);
    button.click();
  }, label);
}

async function clickTestId(cdp, testId) {
  await evaluate(cdp, (testId) => {
    const element = document.querySelector(`[data-testid="${testId}"]`);
    if (!element) throw new Error(`test id not found: ${testId}`);
    element.click();
  }, testId);
}

async function setStoredTheme(cdp, theme) {
  await evaluate(cdp, (theme) => {
    localStorage.setItem('gpufleet-theme', theme);
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
  }, theme);
}

async function waitForTheme(cdp, theme) {
  const deadline = Date.now() + 3000;
  while (Date.now() < deadline) {
    const current = await evaluate(cdp, () => document.documentElement.dataset.theme || '');
    if (current === theme) return;
    await delay(100);
  }
  throw new Error(`timed out waiting for theme ${theme}`);
}

async function assertFleetBoard(cdp) {
  const status = await evaluate(cdp, () => ({
    rowCount: document.querySelectorAll('.fleet-row:not(.fleet-header)').length,
    hasFleetBoard: Boolean(document.querySelector('[data-testid="fleet-board"]')),
    text: document.body.innerText
  }));
  if (!status.hasFleetBoard || status.rowCount < 1) {
    throw new Error(`fleet board missing or empty: ${JSON.stringify(status)}`);
  }
  if (!status.text.includes('GPU Fleet') || !status.text.includes('显存') || !status.text.includes('功耗')) {
    throw new Error('fleet board does not expose compact GPU operations fields');
  }
}

async function visibleText(cdp) {
  return evaluate(cdp, () => document.body.innerText || '');
}

async function waitForText(cdp, expected, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const bodyText = await visibleText(cdp);
    if (expected.every((item) => bodyText.includes(item))) return bodyText;
    await delay(150);
  }
  throw new Error(`timed out waiting for text: ${expected.join(', ')}`);
}

async function waitForLoad(cdp) {
  await cdp.send('Page.getNavigationHistory').catch(() => undefined);
  await delay(750);
}

async function assertNoConsoleErrors(cdp) {
  const logs = await cdp.send('Runtime.evaluate', {
    expression: 'window.__gpufleetConsoleErrors || []',
    returnByValue: true
  });
  const errors = logs.result?.value || [];
  if (errors.length > 0) throw new Error(`browser console errors: ${errors.join('; ')}`);
}

async function screenshot(cdp, file) {
  const result = await cdp.send('Page.captureScreenshot', {
    format: 'png',
    captureBeyondViewport: true,
    fromSurface: true
  });
  await writeFile(file, Buffer.from(result.data, 'base64'));
}

async function evaluate(cdp, fn, arg) {
  const source = `(${fn.toString()})(${JSON.stringify(arg)})`;
  const result = await cdp.send('Runtime.evaluate', {
    expression: source,
    awaitPromise: true,
    returnByValue: true
  });
  if (result.exceptionDetails) {
    const detail = result.exceptionDetails.exception?.description || result.exceptionDetails.text;
    throw new Error(detail);
  }
  return result.result?.value;
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function waitForExit(child) {
  return new Promise((resolve) => {
    if (child.exitCode !== null) return resolve();
    child.once('exit', resolve);
  });
}

function logStep(message) {
  if (process.env.GPUFLEET_VERIFY_DEBUG === '1') {
    console.error(`[verify] ${message}`);
  }
}

class CDPConnection {
  constructor(socket) {
    this.socket = socket;
    this.nextID = 1;
    this.pending = new Map();
    socket.addEventListener('message', (event) => this.handle(event.data));
    socket.addEventListener('error', () => this.rejectAll(new Error('CDP websocket error')));
    socket.addEventListener('close', () => this.rejectAll(new Error('CDP websocket closed')));
  }

  static async open(url) {
    const socket = new WebSocket(url);
    await new Promise((resolve, reject) => {
      socket.addEventListener('open', resolve, { once: true });
      socket.addEventListener('error', reject, { once: true });
    });
    return new CDPConnection(socket);
  }

  send(method, params = {}) {
    const id = this.nextID;
    this.nextID += 1;
    this.socket.send(JSON.stringify({ id, method, params }));
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
    });
  }

  handle(raw) {
    const msg = JSON.parse(raw);
    if (!msg.id || !this.pending.has(msg.id)) return;
    const pending = this.pending.get(msg.id);
    this.pending.delete(msg.id);
    if (msg.error) {
      pending.reject(new Error(`${msg.error.message}: ${msg.error.data || ''}`));
    } else {
      pending.resolve(msg.result || {});
    }
  }

  rejectAll(err) {
    for (const pending of this.pending.values()) {
      pending.reject(err);
    }
    this.pending.clear();
  }

  async close() {
    this.socket.close();
  }
}

const watchdog = setTimeout(() => {
  console.error('frontend verification timed out');
  process.exit(1);
}, 60000);

main()
  .then(() => {
    clearTimeout(watchdog);
  })
  .catch((err) => {
    clearTimeout(watchdog);
    console.error(err?.stack || err);
    process.exitCode = 1;
  });
