#!/usr/bin/env node
// Built Web startup proof. Only Node 24 built-ins and a real Chrome/Chromium.
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { createServer } from 'node:http';
import { readFile, writeFile, mkdir, mkdtemp } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, join, extname, sep } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { clickRetry, pollAfterNavigation } from './web-startup-poll.mjs';

const root = resolve(process.argv[2] ?? 'src/frontend/build/web');
const output = resolve(process.argv[3] ?? 'tmp/web-startup-browser');
const sourceSha = process.env.VOICE_SOURCE_SHA ?? process.env.GITHUB_SHA;
assert.match(sourceSha ?? '', /^[a-f0-9]{40}$/, 'Evidence needs immutable source SHA');
await mkdir(output, { recursive: true });
const evidence = { sourceSha, journeys: [], passed: false };
let mode = 'delay';
let release;
let held;
let failOnce = false;
const server = createServer(async (request, response) => {
  try {
    const pathname = decodeURIComponent(new URL(request.url, 'http://localhost').pathname);
    const file = resolve(root, `.${pathname === '/' ? '/index.html' : pathname}`);
    if (!file.startsWith(root + sep)) throw new Error('Outside root');
    if (pathname.endsWith('/flutter_bootstrap.js') && mode === 'fail' && !failOnce) {
      failOnce = true;
      response.writeHead(503).end();
      return;
    }
    if (pathname.endsWith('/main.dart.js') && mode === 'delay') {
      held = true;
      await new Promise(done => { release = done; });
    }
    response.setHeader('cache-control', 'no-store');
    response.setHeader('content-type', ({ '.js': 'text/javascript', '.html': 'text/html',
      '.json': 'application/json', '.wasm': 'application/wasm', '.png': 'image/png',
      '.ttf': 'font/ttf' })[extname(file)] ?? 'application/octet-stream');
    response.end(await readFile(file));
  } catch { response.writeHead(404).end(); }
});
async function until(description, check, timeout = 60000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    const value = await check();
    if (value) return value;
    await delay(100);
  }
  throw new Error(`Timed out: ${description}`);
}
let chrome, socket;
try {
  await new Promise(done => server.listen(0, '127.0.0.1', done));
  const url = `http://127.0.0.1:${server.address().port}/`;
  const profile = await mkdtemp(join(tmpdir(), 'voice-startup-'));
  chrome = spawn(process.env.CHROME_BIN ?? 'google-chrome', ['--headless=new', '--no-sandbox',
    '--disable-dev-shm-usage', '--no-first-run', '--remote-debugging-port=0',
    `--user-data-dir=${profile}`, 'about:blank'], { stdio: ['ignore', 'ignore', 'ignore'] });
  let launchError;
  chrome.on('error', error => { launchError = error; });
  const port = await until('Chrome startup', async () => {
    if (launchError) throw launchError;
    if (chrome.exitCode !== null) throw new Error(`Chrome exited ${chrome.exitCode}`);
    try { return (await readFile(join(profile, 'DevToolsActivePort'), 'utf8')).split('\n')[0]; }
    catch { return null; }
  });
  evidence.browser = await (await fetch(`http://127.0.0.1:${port}/json/version`)).json();
  socket = new WebSocket(evidence.browser.webSocketDebuggerUrl);
  await new Promise((done, reject) => {
    socket.addEventListener('open', done, { once: true });
    socket.addEventListener('error', reject, { once: true });
  });
  let id = 0;
  const pending = new Map();
  socket.addEventListener('message', ({ data }) => {
    const message = JSON.parse(data);
    const entry = pending.get(message.id);
    if (!entry) return;
    pending.delete(message.id);
    clearTimeout(entry.timer);
    if (message.error) entry.reject(Object.assign(new Error(message.error.message), { code: message.error.code }));
    else entry.done(message.result);
  });
  const send = (method, params = {}, sessionId) => new Promise((done, reject) => {
    const requestId = ++id;
    const timer = setTimeout(() => { pending.delete(requestId); reject(new Error(method)); }, 15000);
    pending.set(requestId, { done, reject, timer });
    socket.send(JSON.stringify({ id: requestId, method, params, sessionId }));
  });
  for (const viewport of [{ name: 'desktop', width: 1280, height: 900, mobile: false },
    { name: 'mobile', width: 390, height: 844, mobile: true }]) {
    const journey = { viewport };
    evidence.journeys.push(journey);
    const { targetId } = await send('Target.createTarget', { url: 'about:blank' });
    const { sessionId } = await send('Target.attachToTarget', { targetId, flatten: true });
    const command = (method, params) => send(method, params, sessionId);
    const evaluate = async expression => {
      const result = await command('Runtime.evaluate', { expression, returnByValue: true });
      assert.equal(result.exceptionDetails, undefined);
      return result.result.value;
    };
    const screenshot = async name => {
      const { data } = await command('Page.captureScreenshot', { format: 'png' });
      await writeFile(join(output, `${viewport.name}-${name}.png`), Buffer.from(data, 'base64'));
    };
    await command('Page.enable');
    await command('Network.enable');
    await command('Network.setCacheDisabled', { cacheDisabled: true });
    await command('Emulation.setDeviceMetricsOverride', { ...viewport, deviceScaleFactor: 1 });
    mode = 'delay'; held = false;
    await command('Page.navigate', { url });
    await until('main entrypoint deliberately held', () => held);
    await until('startup logo decoded', () => evaluate(`(() => {
      const i = document.querySelector('#voice-startup img'); return i?.complete && i.naturalWidth > 0;
    })()`));
    journey.beforeEntrypoint = await evaluate(`(() => {
      const p = document.getElementById('voice-startup'), i = p?.querySelector('img');
      return { panel: !!p, status: document.getElementById('voice-startup-status')?.textContent,
        logoLoaded: !!i?.complete && i.naturalWidth > 0,
        bounds: p ? { width: p.clientWidth, height: p.clientHeight } : null,
        flutterView: !!document.querySelector('flutter-view') };
    })()`);
    assert.equal(journey.beforeEntrypoint.panel, true);
    assert.equal(journey.beforeEntrypoint.logoLoaded, true);
    assert.equal(journey.beforeEntrypoint.flutterView, false);
    assert.ok(journey.beforeEntrypoint.bounds.width <= viewport.width);
    await screenshot('loading');
    release(); release = null;
    await until('real Flutter first rendered frame', () => evaluate(
      `!document.getElementById('voice-startup') && performance.getEntriesByName('voice-startup:first-frame').length === 1`));
    journey.timings = await evaluate(`performance.getEntriesByType('mark')
      .filter(e => e.name.startsWith('voice-startup:')).map(e => ({ name: e.name, startTime: e.startTime }))`);
    await screenshot('ready');
    // A new document fails the bootstrap download, then a trusted pointer click retries it.
    mode = 'fail'; failOnce = false;
    await command('Page.navigate', { url: `${url}?retry-check` });
    await until('failed bootstrap exposes retry', () => evaluate(
      `document.getElementById('voice-startup-retry')?.hidden === false`));
    await screenshot('error');
    const point = await evaluate(`(() => { const r = document.getElementById('voice-startup-retry').getBoundingClientRect();
      return { x: r.x + r.width / 2, y: r.y + r.height / 2 }; })()`);
    await clickRetry(command, point);
    await pollAfterNavigation(() => evaluate(
      `!document.getElementById('voice-startup') && performance.getEntriesByName('voice-startup:first-frame').length === 1`));
    journey.retryRendered = true;
    await screenshot('retry-ready');
    await send('Target.closeTarget', { targetId });
  }
  evidence.passed = true;
  console.log('PASS: built Web cold startup, logo, first frame, bootstrap failure and trusted retry in desktop/mobile Chrome');
} catch (error) {
  evidence.error = error.stack ?? String(error);
  console.error(evidence.error);
  process.exitCode = 1;
} finally {
  release?.();
  await writeFile(join(output, 'evidence.json'), JSON.stringify(evidence, null, 2) + '\n');
  socket?.close();
  chrome?.kill('SIGTERM');
  server.closeAllConnections();
  await new Promise(done => server.close(done));
}
