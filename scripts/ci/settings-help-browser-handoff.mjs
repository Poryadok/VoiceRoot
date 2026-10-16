#!/usr/bin/env node
// Actual HelpSheet/url_launcher browser acceptance. Node 24 built-ins only.
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createWriteStream } from 'node:fs';
import { mkdir, mkdtemp, readFile, writeFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { extname, join, resolve, sep } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';

const root = resolve(process.argv[2] ?? 'src/frontend/build/help-launcher-web');
const output = resolve(process.argv[3] ?? 'tmp/settings-help-browser');
const sourceSha = process.env.VOICE_SOURCE_SHA ?? process.env.GITHUB_SHA;
const checkoutSha = process.env.GITHUB_SHA;
assert.match(sourceSha ?? '', /^[a-f0-9]{40}$/, 'Source SHA must bind browser evidence');
assert.match(checkoutSha ?? '', /^[a-f0-9]{40}$/, 'GITHUB_SHA must bind the tested checkout');
await mkdir(output, { recursive: true });
const evidence = { sourceSha, checkoutSha, journeys: [], passed: false };
const mime = { '.html': 'text/html', '.js': 'text/javascript', '.json': 'application/json',
  '.wasm': 'application/wasm', '.woff2': 'font/woff2', '.png': 'image/png' };
const server = createServer(async (request, response) => {
  try {
    const pathname = decodeURIComponent(new URL(request.url, 'http://localhost').pathname);
    const file = resolve(root, `.${pathname === '/' ? '/index.html' : pathname}`);
    if (!file.startsWith(`${root}${sep}`)) throw new Error('Outside fixture root');
    response.setHeader('content-type', mime[extname(file)] ?? 'application/octet-stream');
    response.setHeader('cache-control', 'no-store');
    response.end(await readFile(file));
  } catch {
    response.writeHead(404).end();
  }
});

async function until(description, check, timeout = 30000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const value = await check();
    if (value) return value;
    await delay(150);
  }
  throw new Error(`Timed out: ${description}`);
}

class Cdp {
  constructor(socket) {
    this.socket = socket;
    this.nextId = 0;
    this.pending = new Map();
    this.events = [];
    socket.addEventListener('message', ({ data }) => {
      const message = JSON.parse(data);
      if (message.id) {
        const pending = this.pending.get(message.id);
        if (!pending) return;
        this.pending.delete(message.id);
        clearTimeout(pending.timer);
        if (message.error) pending.reject(new Error(JSON.stringify(message.error)));
        else pending.resolve(message.result);
      } else if (['Page.windowOpen', 'Runtime.exceptionThrown',
        'Target.targetCreated', 'Target.targetInfoChanged'].includes(message.method)) {
        this.events.push(message);
      }
    });
  }

  send(method, params = {}, sessionId) {
    const id = ++this.nextId;
    return new Promise((resolveMessage, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`CDP timeout: ${method}`));
      }, 10000);
      this.pending.set(id, { resolve: resolveMessage, reject, timer });
      this.socket.send(JSON.stringify({ id, method, params, sessionId }));
    });
  }
}

let chrome;
let cdp;
let captureFailure;
try {
  await new Promise((done) => server.listen(0, '127.0.0.1', done));
  const fixtureUrl = `http://127.0.0.1:${server.address().port}/`;
  evidence.fixtureUrl = fixtureUrl;
  const profile = await mkdtemp(join(tmpdir(), 'voice-help-chrome-'));
  const chromeLog = createWriteStream(join(output, 'chrome.log'));
  chrome = spawn(process.env.CHROME_BIN ?? 'google-chrome', [
    '--headless=new', '--no-sandbox', '--disable-dev-shm-usage', '--no-first-run',
    '--no-default-browser-check', '--remote-debugging-port=0',
    `--user-data-dir=${profile}`, 'about:blank',
  ], { stdio: ['ignore', 'ignore', 'pipe'] });
  let launchError;
  chrome.on('error', (error) => { launchError = error; });
  chrome.stderr.pipe(chromeLog);
  const port = await until('Chrome DevToolsActivePort', async () => {
    if (launchError) throw launchError;
    if (chrome.exitCode !== null) throw new Error(`Chrome exited ${chrome.exitCode}`);
    try { return (await readFile(join(profile, 'DevToolsActivePort'), 'utf8')).split('\n')[0]; }
    catch { return null; }
  });
  const version = await (await fetch(`http://127.0.0.1:${port}/json/version`)).json();
  evidence.browser = version;
  const socket = new WebSocket(version.webSocketDebuggerUrl);
  await new Promise((done, reject) => {
    socket.addEventListener('open', done, { once: true });
    socket.addEventListener('error', reject, { once: true });
  });
  cdp = new Cdp(socket);
  await cdp.send('Target.setDiscoverTargets', { discover: true });

  for (const viewport of [{ name: 'h', width: 1280, height: 900 },
    { name: 'v', width: 390, height: 844 }]) {
    const journey = { viewport, handoffs: [] };
    evidence.journeys.push(journey);
    const { targetId } = await cdp.send('Target.createTarget', { url: 'about:blank' });
    journey.targetId = targetId;
    const { sessionId } = await cdp.send('Target.attachToTarget', { targetId, flatten: true });
    const send = (method, params) => cdp.send(method, params, sessionId);
    const evaluate = async (expression) => {
      const result = await send('Runtime.evaluate', { expression, returnByValue: true });
      assert.equal(result.exceptionDetails, undefined, 'DOM observation must not throw');
      return result.result.value;
    };
    captureFailure = async () => {
      const { data } = await send('Page.captureScreenshot', { format: 'png' });
      await writeFile(join(output, `${viewport.name}-failure.png`), Buffer.from(data, 'base64'));
    };
    await send('Page.enable');
    await send('Runtime.enable');
    await send('Emulation.setDeviceMetricsOverride', {
      width: viewport.width, height: viewport.height, deviceScaleFactor: 1, mobile: false,
    });
    await send('Page.navigate', { url: fixtureUrl });
    const labels = () => evaluate(`Array.from(document.querySelectorAll('[aria-label]'))
      .map(e => e.getAttribute('aria-label'))`);
    const locate = (label) => evaluate(`(() => {
      const matches = Array.from(document.querySelectorAll('[aria-label]'))
        .filter(e => e.getAttribute('aria-label').trim() === ${JSON.stringify(label)});
      for (const e of matches.reverse()) {
        const r = e.getBoundingClientRect();
        const x = r.x + r.width / 2, y = r.y + r.height / 2;
        if (r.width > 0 && r.height > 0 && x > 0 && x < innerWidth && y > 0 && y < innerHeight)
          return { x, y, width: r.width, height: r.height, role: e.getAttribute('role') };
      }
      return null;
    })()`);
    const wheel = async (deltaY) => {
      await send('Input.dispatchMouseEvent', { type: 'mouseWheel',
        x: viewport.width / 2, y: viewport.height * .75, deltaX: 0, deltaY });
      await delay(250);
    };
    const find = async (label) => {
      for (let attempt = 0; attempt < 16; attempt++) {
        const bounds = await locate(label);
        if (bounds) return bounds;
        if (attempt < 3) await delay(350); // Sheet/semantics startup animation.
        else await wheel(350);
      }
      journey.labelsAtFailure = await labels();
      throw new Error(`Visible semantics missing: ${label} (${viewport.name})`);
    };
    const click = async (label) => {
      const bounds = await find(label);
      await send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: bounds.x, y: bounds.y });
      await send('Input.dispatchMouseEvent', { type: 'mousePressed',
        x: bounds.x, y: bounds.y, button: 'left', buttons: 1, clickCount: 1 });
      await send('Input.dispatchMouseEvent', { type: 'mouseReleased',
        x: bounds.x, y: bounds.y, button: 'left', buttons: 0, clickCount: 1 });
      return bounds;
    };
    await until('fixture startup and enabled semantics', () => locate('Open Settings Help smoke'));
    await click('Open Settings Help smoke');
    await click('Help');
    await find('Project documentation');
    const screenshot = async (name) => {
      const { data } = await send('Page.captureScreenshot', { format: 'png' });
      await writeFile(join(output, `${viewport.name}-${name}.png`), Buffer.from(data, 'base64'));
    };
    await screenshot('help-before');
    for (const [label, expectedUrl, suffix] of [
      ['Project documentation', 'https://github.com/Poryadok/VoiceRoot/blob/master/README.md', 'readme'],
      ['Contact support', 'https://github.com/Poryadok/VoiceRoot/issues', 'issues'],
    ]) {
      const before = (await cdp.send('Target.getTargets')).targetInfos;
      const beforeIds = new Set(before.map((target) => target.targetId));
      const eventStart = cdp.events.length;
      const bounds = await click(label);
      const target = await until(`${suffix} actual new target URL`, async () => {
        const targets = (await cdp.send('Target.getTargets')).targetInfos;
        return targets.find((entry) => !beforeIds.has(entry.targetId) && entry.type === 'page' &&
          entry.url === expectedUrl);
      });
      await delay(500);
      const after = (await cdp.send('Target.getTargets')).targetInfos;
      const newPages = after.filter((entry) => !beforeIds.has(entry.targetId) && entry.type === 'page');
      assert.equal(newPages.length, 1, `${suffix}: exactly one actual new tab`);
      const createdPageIds = [...new Set(cdp.events.slice(eventStart)
        .filter((event) => event.method === 'Target.targetCreated' &&
          event.params.targetInfo.type === 'page' && !beforeIds.has(event.params.targetInfo.targetId))
        .map((event) => event.params.targetInfo.targetId))];
      assert.deepEqual(createdPageIds, [target.targetId], `${suffix}: no transient extra tab`);
      const windowOpen = cdp.events.slice(eventStart).filter((event) =>
        event.sessionId === sessionId && event.method === 'Page.windowOpen');
      assert.equal(windowOpen.length, 1, `${suffix}: exactly one windowOpen`);
      assert.equal(windowOpen[0].params.url, expectedUrl);
      assert.equal(windowOpen[0].params.userGesture, true, `${suffix}: trusted pointer gesture`);
      const opener = after.find((entry) => entry.targetId === targetId);
      assert.equal(opener?.url, fixtureUrl, `${suffix}: original Help tab retained`);
      const handoff = { label, expectedUrl, bounds, windowOpen: windowOpen[0].params,
        target, opener, newPageCount: newPages.length, createdPageIds, launchErrorScan: [] };
      journey.handoffs.push(handoff);
      await cdp.send('Target.closeTarget', { targetId: target.targetId });
      await send('Page.bringToFront');
      await find('Contact support');
      // Flutter may omit clipped semantics. Cover the error row below both buttons.
      for (let step = 0; step <= 3; step++) {
        if (step) await wheel(120);
        const visibleLabels = await labels();
        handoff.launchErrorScan.push(visibleLabels);
        assert.ok(!visibleLabels.some((text) => text.includes('Could not open link.')),
          `${suffix}: no Help launch error in settled region ${step}`);
      }
      await wheel(-360);
      await find('Contact support');
      await screenshot(`help-after-${suffix}`);
    }
    const exceptions = cdp.events.filter((event) =>
      event.sessionId === sessionId && event.method === 'Runtime.exceptionThrown');
    journey.exceptions = exceptions;
    assert.equal(exceptions.length, 0, 'No uncaught fixture exceptions');
    await cdp.send('Target.closeTarget', { targetId });
    captureFailure = null;
  }
  evidence.passed = true;
  console.log(`PASS: Help README/Issues trusted-click handoff in H/V; source ${sourceSha}`);
} catch (error) {
  evidence.error = error.stack ?? String(error);
  process.exitCode = 1;
  console.error(evidence.error);
  try { await captureFailure?.(); }
  catch (captureError) { evidence.captureError = String(captureError); }
} finally {
  evidence.events = cdp?.events ?? [];
  await writeFile(join(output, 'evidence.json'), `${JSON.stringify(evidence, null, 2)}\n`);
  cdp?.socket.close();
  chrome?.kill('SIGTERM');
  server.closeAllConnections();
  await new Promise((done) => server.close(done));
}
