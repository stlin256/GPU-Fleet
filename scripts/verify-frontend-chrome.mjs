import { mkdir, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { spawn } from 'node:child_process';
import net from 'node:net';

async function main() {
  const options = parseArgs(process.argv.slice(2));
  const chromePath = options.chrome || 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe';
  const targetURL = required(options.url, '--url');
  const password = required(options.password, '--password');
  const outDir = options.out || path.resolve('logs', `frontend-verify-${Date.now()}`);
  const minFleetCards = Number.parseInt(options['min-fleet-cards'] || '1', 10);
  const expectedVersion = options['expected-version'] || 'v0.1.9';
  const requireOfflineMask = options['require-offline-mask'] === 'true' || options['require-offline-mask'] === '1';
  const requireDualDevice = options['require-dual-device'] === 'true' || options['require-dual-device'] === '1';
  const screenshotSizes = {};

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

      await waitForLoad(cdp, targetURL);
      await setStoredTheme(cdp, 'light');
      await cdp.send('Page.reload', { ignoreCache: true });
      await waitForLoad(cdp, targetURL);
      logStep('logging in');
      await login(cdp, password);
      await waitForText(cdp, ['GPUFleet', text('GPU resource overview'), text('fleet board'), text('fleet live')], 12000);
      await assertNoConsoleErrors(cdp);
      const fleetStatus = await assertFleetBoard(cdp, { minFleetCards, requireOfflineMask, requireDualDevice });
      const tooltipStatus = await assertTrendTooltip(cdp);
      await waitForTheme(cdp, 'light');
      logStep('capturing desktop light overview');
      screenshotSizes.desktop_overview = await screenshot(cdp, path.join(outDir, 'desktop-overview.png'));

      logStep('checking dark theme');
      await clickTestId(cdp, 'theme-toggle');
      await waitForTheme(cdp, 'dark');
      screenshotSizes.desktop_overview_dark = await screenshot(cdp, path.join(outDir, 'desktop-overview-dark.png'));

      logStep('checking reload session restore');
      await cdp.send('Page.reload', { ignoreCache: true });
      await waitForLoad(cdp, targetURL);
      await waitForText(cdp, ['GPUFleet', text('GPU resource overview'), text('fleet board')], 12000);
      const dashboardAfterReload = await visibleText(cdp);
      if (dashboardAfterReload.includes(text('login panel'))) {
        throw new Error('session was not restored after page reload');
      }
      await waitForTheme(cdp, 'dark');

      logStep('checking devices view');
      await clickButton(cdp, text('devices'));
      await waitForText(cdp, [text('device management'), text('register device')], 5000);
      screenshotSizes.desktop_devices = await screenshot(cdp, path.join(outDir, 'desktop-devices.png'));

      logStep('checking energy view');
      await clickButton(cdp, text('energy'));
      await waitForText(cdp, [text('thermal energy'), text('current power'), text('gpu energy ranking'), text('energy diagnostics')], 5000);
      const energyLayout = await assertEnergyPage(cdp);
      screenshotSizes.desktop_energy = await screenshot(cdp, path.join(outDir, 'desktop-energy.png'));

      logStep('checking settings view');
      await clickButton(cdp, text('settings'));
      await waitForText(cdp, [text('service settings'), text('service status'), text('password change'), text('port config'), text('https certificate'), text('energy display'), text('database download'), text('download diagnostics'), text('online update'), text('setup wizard'), text('release info'), text('latest changelog'), expectedVersion, 'stlin256', 'https://github.com/stlin256/GPU-Fleet'], 5000);
      const settingsLayout = await evaluate(cdp, () => ({
        statCount: document.querySelectorAll('[data-testid="setting-stat"]').length,
        operationCount: document.querySelectorAll('.setting-operation').length,
        passwordPanel: Boolean(document.querySelector('[data-testid="settings-password"]')),
        portPanel: Boolean(document.querySelector('[data-testid="settings-port"]')),
        certPanel: Boolean(document.querySelector('[data-testid="settings-certificate"]')),
        energyPanel: Boolean(document.querySelector('[data-testid="settings-energy-display"]')),
        guestPanel: Boolean(document.querySelector('[data-testid="settings-guest"]')),
        restartPanel: Boolean(document.querySelector('[data-testid="settings-restart"]')),
        databasePanel: Boolean(document.querySelector('[data-testid="settings-database"]')),
        diskReservePanel: Boolean(document.querySelector('[data-testid="settings-disk-reserve"]')),
        updatePanel: Boolean(document.querySelector('[data-testid="settings-update"]')),
        projectPanel: Boolean(document.querySelector('[data-testid="settings-project"]')),
        changelogPanel: Boolean(document.querySelector('[data-testid="settings-changelog"]')),
        brandLogoCount: document.querySelectorAll('.brand-mark').length,
        databaseLink: document.querySelector('[data-testid="settings-database"] a')?.getAttribute('href') || '',
        diagnosticsLink: Array.from(document.querySelectorAll('[data-testid="settings-database"] a')).map((item) => item.getAttribute('href') || '').find((href) => href.includes('/api/v1/admin/diagnostics/download')) || '',
        projectLink: document.querySelector('[data-testid="settings-project"] a[href="https://github.com/stlin256/GPU-Fleet"]')?.getAttribute('href') || '',
        hasSettingsPage: Boolean(document.querySelector('[data-testid="settings-page"]')),
        bodyText: document.body.innerText
      }));
      if (!settingsLayout.hasSettingsPage || settingsLayout.statCount < 4 || settingsLayout.operationCount < 10) {
        throw new Error(`settings page is incomplete: ${JSON.stringify(settingsLayout)}`);
      }
      if (!settingsLayout.passwordPanel || !settingsLayout.portPanel || !settingsLayout.certPanel || !settingsLayout.energyPanel || !settingsLayout.guestPanel || !settingsLayout.restartPanel || !settingsLayout.databasePanel || !settingsLayout.diskReservePanel || !settingsLayout.updatePanel || !settingsLayout.projectPanel || !settingsLayout.changelogPanel || !settingsLayout.databaseLink.includes('/api/v1/admin/database/download') || !settingsLayout.diagnosticsLink.includes('/api/v1/admin/diagnostics/download') || settingsLayout.projectLink !== 'https://github.com/stlin256/GPU-Fleet') {
        throw new Error(`settings page does not expose operational controls: ${JSON.stringify(settingsLayout)}`);
      }
      if (settingsLayout.brandLogoCount < 1 || !settingsLayout.bodyText.includes('版本与变更') || !settingsLayout.bodyText.includes('最近变更') || !settingsLayout.bodyText.includes(expectedVersion) || !settingsLayout.bodyText.includes('stlin256')) {
        throw new Error(`release, brand, or repository attribution is missing: ${JSON.stringify(settingsLayout)}`);
      }
      await assertSettingsDialogs(cdp);
      screenshotSizes.desktop_settings = await screenshot(cdp, path.join(outDir, 'desktop-settings.png'));

      logStep('checking mobile overview');
      await clickButton(cdp, text('overview'));
      await cdp.send('Emulation.setDeviceMetricsOverride', {
        width: 390,
        height: 900,
        deviceScaleFactor: 1,
        mobile: true
      });
      await cdp.send('Page.reload', { ignoreCache: true });
      await waitForLoad(cdp, targetURL);
      await waitForText(cdp, [text('GPU resource overview'), text('fleet board')], 12000);
      screenshotSizes.mobile_overview = await screenshot(cdp, path.join(outDir, 'mobile-overview.png'));
      const mobileOverviewLayout = await evaluate(cdp, () => ({
        width: window.innerWidth,
        height: window.innerHeight,
        scrollWidth: document.documentElement.scrollWidth,
        fleetCardCount: document.querySelectorAll('[data-testid="fleet-gpu-card"]').length,
        trendCount: document.querySelectorAll('[data-testid="gpu-trend-tile"]').length,
        offlineMaskCount: document.querySelectorAll('.offline-mask').length,
        navButtonCount: document.querySelectorAll('.sidebar nav button').length,
        navPosition: getComputedStyle(document.querySelector('.sidebar')).position,
        navBottom: Math.round(document.querySelector('.sidebar').getBoundingClientRect().bottom),
        theme: document.documentElement.dataset.theme || '',
        bodyText: document.body.innerText
      }));
      if (mobileOverviewLayout.scrollWidth > mobileOverviewLayout.width + 2) {
        throw new Error(`mobile overview overflows horizontally: ${mobileOverviewLayout.scrollWidth} > ${mobileOverviewLayout.width}`);
      }
      if (mobileOverviewLayout.fleetCardCount < 1) {
        throw new Error('fleet overview GPU cards were not rendered in mobile browser');
      }
      if (mobileOverviewLayout.navButtonCount < 5 || mobileOverviewLayout.navPosition !== 'fixed' || Math.abs(mobileOverviewLayout.navBottom - mobileOverviewLayout.height) > 2) {
        throw new Error(`mobile bottom navigation is not fixed at the viewport bottom: ${JSON.stringify(mobileOverviewLayout)}`);
      }

      logStep('checking mobile GPU view');
      await clickButton(cdp, text('gpu'));
      await waitForText(cdp, [text('gpu monitoring')], 5000);
      screenshotSizes.mobile_gpu = await screenshot(cdp, path.join(outDir, 'mobile-gpu.png'));

      const layout = await evaluate(cdp, () => ({
        width: window.innerWidth,
        scrollWidth: document.documentElement.scrollWidth,
        title: document.querySelector('h1')?.textContent || '',
        cardCount: document.querySelectorAll('.gpu-card').length,
        detailTrendCount: document.querySelectorAll('.gpu-detail-trend-grid [data-testid="gpu-trend-tile"]').length,
        meterCount: document.querySelectorAll('.meter').length,
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
      if (layout.detailTrendCount < layout.cardCount * 4) {
        throw new Error(`GPU detail trend charts were not rendered: ${JSON.stringify(layout)}`);
      }
      if (layout.meterCount > 0) {
        throw new Error(`legacy meter bars are still visible on GPU detail page: ${JSON.stringify(layout)}`);
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
          desktop_energy: path.join(outDir, 'desktop-energy.png'),
          desktop_settings: path.join(outDir, 'desktop-settings.png'),
          mobile_overview: path.join(outDir, 'mobile-overview.png'),
          mobile_gpu: path.join(outDir, 'mobile-gpu.png')
        },
        layout: {
          width: layout.width,
          scrollWidth: layout.scrollWidth,
          cardCount: layout.cardCount,
          fleetCardCount: mobileOverviewLayout.fleetCardCount,
          fleetTrendCount: mobileOverviewLayout.trendCount,
          offlineMaskCount: mobileOverviewLayout.offlineMaskCount,
          mobileNavButtonCount: mobileOverviewLayout.navButtonCount,
          mobileNavPosition: mobileOverviewLayout.navPosition,
          dualDeviceCardCount: fleetStatus.dualDeviceCardCount,
          dualDeviceColorMatched: fleetStatus.dualDeviceColorMatched,
          distinctDeviceColorCount: fleetStatus.distinctDeviceColorCount,
          sparkTooltipCount: tooltipStatus.count,
          energyMetricCount: energyLayout.metricCount,
          energyTrendCount: energyLayout.trendCount,
          energyRangeButtonCount: energyLayout.rangeButtonCount,
          energySettingsPanel: settingsLayout.energyPanel,
          detailTrendCount: layout.detailTrendCount,
          meterCount: layout.meterCount,
          settingsStatCount: settingsLayout.statCount,
          settingsOperationCount: settingsLayout.operationCount,
          settingsChangelogPanel: settingsLayout.changelogPanel,
          settingsUpdatePanel: settingsLayout.updatePanel,
          settingsDiagnosticsLink: settingsLayout.diagnosticsLink,
          screenshotSizes,
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
    'service settings': '\u670d\u52a1\u8bbe\u7f6e',
    'service status': '\u670d\u52a1\u72b6\u6001',
    'password change': '\u5bc6\u7801\u66f4\u6539',
    'port config': '\u7aef\u53e3\u914d\u7f6e',
    'https certificate': '\u0048\u0054\u0054\u0050\u0053 \u8bc1\u4e66',
    'database download': '\u6570\u636e\u5e93\u4e0b\u8f7d',
    'download diagnostics': '\u4e0b\u8f7d\u8bca\u65ad\u5305',
    'online update': '\u5728\u7ebf\u66f4\u65b0',
    'setup wizard': '\u914d\u7f6e\u5f15\u5bfc',
    'release info': '\u7248\u672c\u4e0e\u53d8\u66f4',
    'latest changelog': '\u6700\u8fd1\u53d8\u66f4',
    'guest records': '\u8bbf\u5ba2\u8bb0\u5f55',
    'restart service': '\u91cd\u542f\u670d\u52a1',
    overview: '\u603b\u89c8',
    devices: '\u8bbe\u5907',
    energy: '\u80fd\u8017',
    gpu: '\u0047\u0050\u0055',
    settings: '\u8bbe\u7f6e',
    'gpu monitoring': '\u0047\u0050\u0055 \u76d1\u63a7',
    'thermal energy': '\u70ed\u80fd\u4e0e\u80fd\u6e90',
    'current power': '\u5f53\u524d\u529f\u7387',
    'gpu energy ranking': '\u0047\u0050\u0055 \u80fd\u8017\u6392\u884c',
    'energy diagnostics': '\u80fd\u6e90\u8bca\u65ad',
    'energy display': '\u80fd\u8017\u5c55\u793a',
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

async function login(cdp, pass) {
  await waitForText(cdp, [text('login panel')], 10000);
  await evaluate(cdp, (pass) => {
    const setValue = (input, value) => {
      const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value').set;
      setter.call(input, value);
      input.dispatchEvent(new Event('input', { bubbles: true }));
    };
    const inputs = Array.from(document.querySelectorAll('input'));
    if (inputs.length < 1) throw new Error('login input not found');
    setValue(inputs[0], pass);
    document.querySelector('form').requestSubmit();
  }, pass);
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

async function clickWithinTestId(cdp, testId, label) {
  await evaluate(cdp, ({ testId, label }) => {
    const root = document.querySelector(`[data-testid="${testId}"]`);
    if (!root) throw new Error(`test id not found: ${testId}`);
    const button = Array.from(root.querySelectorAll('button')).find((item) =>
      (item.textContent || '').includes(label)
    );
    if (!button) throw new Error(`button not found in ${testId}: ${label}`);
    button.click();
  }, { testId, label });
}

async function clickSelector(cdp, selector) {
  await evaluate(cdp, (selector) => {
    const element = document.querySelector(selector);
    if (!element) throw new Error(`selector not found: ${selector}`);
    element.click();
  }, selector);
}

async function assertVisibleTestId(cdp, testId) {
  const visible = await evaluate(cdp, (testId) => {
    const element = document.querySelector(`[data-testid="${testId}"]`);
    if (!element) return false;
    const rect = element.getBoundingClientRect();
    const style = getComputedStyle(element);
    return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
  }, testId);
  if (!visible) throw new Error(`expected visible test id: ${testId}`);
}

async function closeTopDialog(cdp) {
  await cdp.send('Input.dispatchKeyEvent', { type: 'keyDown', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 });
  await cdp.send('Input.dispatchKeyEvent', { type: 'keyUp', key: 'Escape', code: 'Escape', windowsVirtualKeyCode: 27 });
  await delay(150);
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

async function assertFleetBoard(cdp, checks) {
  const status = await evaluate(cdp, () => ({
    cardCount: document.querySelectorAll('[data-testid="fleet-gpu-card"]').length,
    trendCount: document.querySelectorAll('[data-testid="gpu-trend-tile"]').length,
    offlineMaskCount: document.querySelectorAll('.offline-mask').length,
    hasFleetBoard: Boolean(document.querySelector('[data-testid="fleet-board"]')),
    hasSparkline: Boolean(document.querySelector('.sparkline .spark-line')),
    hasSparklineWrap: Boolean(document.querySelector('[data-testid="gpu-trend-tile"] .sparkline-wrap')),
    deviceColorGroups: Array.from(document.querySelectorAll('[data-testid="fleet-gpu-card"]')).reduce((groups, card) => {
      const device = card.getAttribute('data-device-id') || card.querySelector('.fleet-device-cell strong')?.textContent?.trim() || '';
      const color = card.getAttribute('data-device-color') || getComputedStyle(card).borderTopColor || '';
      if (!device) return groups;
      const existing = groups.find((item) => item.device === device);
      if (existing) {
        existing.colors.push(color);
        existing.count += 1;
      } else {
        groups.push({ device, colors: [color], count: 1 });
      }
      return groups;
    }, []),
    dualDeviceCardCount: Math.max(
      0,
      ...Array.from(document.querySelectorAll('[data-testid="fleet-gpu-card"]')).reduce((counts, card) => {
        const device = card.querySelector('.fleet-device-cell strong')?.textContent?.trim() || '';
        if (device) counts.set(device, (counts.get(device) || 0) + 1);
        return counts;
      }, new Map()).values()
    ),
    text: document.body.innerText
  }));
  status.dualDeviceColorMatched = status.deviceColorGroups.some((group) => group.count >= 2 && new Set(group.colors).size === 1);
  status.sameDeviceColorConsistent = status.deviceColorGroups.every((group) => new Set(group.colors).size === 1);
  status.distinctDeviceColorCount = new Set(status.deviceColorGroups.map((group) => group.colors[0])).size;
  if (!status.hasFleetBoard || status.cardCount < checks.minFleetCards || status.trendCount < status.cardCount * 4 || !status.hasSparkline) {
    throw new Error(`fleet board missing or empty: ${JSON.stringify(status)}`);
  }
  if (!status.hasSparklineWrap) {
    throw new Error(`fleet trend charts are missing interactive hover wrappers: ${JSON.stringify(status)}`);
  }
  if (checks.requireOfflineMask && status.offlineMaskCount < 1) {
    throw new Error(`offline mask was required but not found: ${JSON.stringify(status)}`);
  }
  if (checks.requireDualDevice && status.dualDeviceCardCount < 2) {
    throw new Error(`same-device multi-GPU cards were required but not found: ${JSON.stringify(status)}`);
  }
  if (checks.requireDualDevice && !status.dualDeviceColorMatched) {
    throw new Error(`same-device GPU cards do not share a border color: ${JSON.stringify(status)}`);
  }
  if (!status.sameDeviceColorConsistent || status.distinctDeviceColorCount < Math.min(2, status.deviceColorGroups.length)) {
    throw new Error(`device border colors are not grouped by device: ${JSON.stringify(status)}`);
  }
  if (!status.text.includes('GPU Fleet') || !status.text.includes('GPU 利用率') || !status.text.includes('显存') || !status.text.includes('功耗')) {
    throw new Error('fleet board does not expose trend chart GPU operation fields');
  }
  return status;
}

async function assertTrendTooltip(cdp) {
  const target = await evaluate(cdp, () => {
    const wrap = document.querySelector('[data-testid="gpu-trend-tile"] .sparkline-wrap');
    if (!wrap) return null;
    const rect = wrap.getBoundingClientRect();
    return { x: rect.left + rect.width * 0.72, y: rect.top + rect.height * 0.52 };
  });
  if (!target) throw new Error('sparkline wrapper was not found for tooltip verification');
  await cdp.send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: target.x, y: target.y, button: 'none' });
  await delay(200);
  const status = await evaluate(cdp, () => {
    const tips = Array.from(document.querySelectorAll('[data-testid="spark-tooltip"]'));
    const visible = tips.find((tip) => {
      const rect = tip.getBoundingClientRect();
      const style = getComputedStyle(tip);
      return rect.width > 0 && rect.height > 0 && style.display !== 'none' && style.visibility !== 'hidden';
    });
    return {
      count: tips.length,
      visibleText: visible?.textContent || ''
    };
  });
  if (status.count < 1 || !/\d/.test(status.visibleText)) {
    throw new Error(`sparkline tooltip did not show a numeric value: ${JSON.stringify(status)}`);
  }
  return status;
}

async function assertEnergyPage(cdp) {
  const before = await evaluate(cdp, () => {
    const root = document.querySelector('[data-view="energy"]');
    return {
      hasRoot: Boolean(root),
      hasEnergyPage: Boolean(document.querySelector('[data-testid="energy-page"]')),
      metricCount: document.querySelectorAll('[data-testid="energy-page"] .metric').length,
      trendCount: document.querySelectorAll('.energy-trend-panel [data-testid="gpu-trend-tile"]').length,
      rangeButtonCount: root ? root.querySelectorAll('.segmented-control button').length : 0,
      hasDiagnostics: Boolean(document.querySelector('.energy-diagnostics-panel')),
      hasRanking: Boolean(document.querySelector('.energy-gpu-panel')),
      controlWords: /\u98ce\u6247|\u9891\u7387|\u529f\u8017\u5899|\u6682\u505c\u4efb\u52a1|\u6740\u8fdb\u7a0b/.test(document.body.innerText || ''),
      bodyText: document.body.innerText
    };
  });
  if (!before.hasRoot || !before.hasEnergyPage || before.metricCount < 5 || before.trendCount < 3 || before.rangeButtonCount < 3 || !before.hasDiagnostics || !before.hasRanking || before.controlWords) {
    throw new Error(`energy page is incomplete or exposes control wording: ${JSON.stringify(before)}`);
  }

  for (const label of ['7D', '30D']) {
    await evaluate(cdp, (label) => {
      const root = document.querySelector('[data-view="energy"]');
      if (!root) throw new Error('energy view root not found');
      const button = Array.from(root.querySelectorAll('.segmented-control button')).find((item) => (item.textContent || '').includes(label));
      if (!button) throw new Error(`energy range button not found: ${label}`);
      button.click();
    }, label);
    await delay(200);
    const activeLabel = await evaluate(cdp, () => {
      const active = document.querySelector('[data-view="energy"] .segmented-control button.active');
      return active?.textContent || '';
    });
    if (!activeLabel.includes(label)) {
      throw new Error(`energy range did not switch to ${label}: ${activeLabel}`);
    }
  }

  return before;
}

async function assertSettingsDialogs(cdp) {
  await clickSelector(cdp, '.changelog-toggle');
  await assertVisibleTestId(cdp, 'changelog-dialog');
  await closeTopDialog(cdp);

  await clickWithinTestId(cdp, 'settings-guest', text('guest records'));
  await assertVisibleTestId(cdp, 'guest-records-dialog');
  await closeTopDialog(cdp);

  await clickWithinTestId(cdp, 'settings-restart', text('restart service'));
  await assertVisibleTestId(cdp, 'restart-confirm-dialog');
  await closeTopDialog(cdp);
}

async function visibleText(cdp) {
  return evaluate(cdp, () => document.body.innerText || '');
}

async function waitForText(cdp, expected, timeoutMs) {
  const deadline = Date.now() + timeoutMs;
  let lastBodyText = '';
  while (Date.now() < deadline) {
    const bodyText = await visibleText(cdp);
    lastBodyText = bodyText;
    if (expected.every((item) => bodyText.includes(item))) return bodyText;
    await delay(150);
  }
  throw new Error(`timed out waiting for text: ${expected.join(', ')}. Current text: ${lastBodyText.slice(0, 800)}`);
}

async function waitForLoad(cdp, expectedURL) {
  const deadline = Date.now() + 12000;
  let last = {};
  while (Date.now() < deadline) {
    last = await evaluate(cdp, () => ({
      href: location.href,
      readyState: document.readyState,
      hasStorage: location.protocol === 'http:' || location.protocol === 'https:'
    })).catch((err) => ({ error: err.message }));
    if (
      last.hasStorage &&
      (!expectedURL || String(last.href || '').startsWith(expectedURL)) &&
      (last.readyState === 'interactive' || last.readyState === 'complete')
    ) {
      await delay(300);
      return;
    }
    await delay(120);
  }
  throw new Error(`timed out waiting for target page load: ${JSON.stringify(last)}`);
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
  const bytes = Buffer.from(result.data, 'base64');
  if (bytes.length < 2000) {
    throw new Error(`screenshot appears blank or truncated: ${file} (${bytes.length} bytes)`);
  }
  await writeFile(file, bytes);
  return bytes.length;
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
