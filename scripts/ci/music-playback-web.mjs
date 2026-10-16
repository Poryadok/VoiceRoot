// Hosted Chrome integration: real package WASM/worker and application decoder.
import { createServer } from 'node:http';
import { spawn } from 'node:child_process';
import { mkdtemp, readFile, realpath, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, sep, extname, join } from 'node:path';
import { once } from 'node:events';

const root = await realpath(process.argv[2]);
const profile = await mkdtemp(join(tmpdir(), 'voice-music-chrome-'));
const mime = { '.js': 'text/javascript', '.wasm': 'application/wasm', '.html': 'text/html', '.json': 'application/json' };
const server = createServer(async (req, res) => {
  try {
    const path = await realpath(resolve(root, '.' + decodeURIComponent(new URL(req.url, 'http://localhost').pathname)));
    if (!path.startsWith(root + sep)) throw new Error('path');
    const body = await readFile(path);
    res.writeHead(200, { 'Content-Type': mime[extname(path)] || 'application/octet-stream', 'Cache-Control': 'no-store' });
    res.end(body);
  } catch { res.writeHead(404); res.end(); }
});
server.listen(0, '127.0.0.1');
await once(server, 'listening');
let chrome, socket, chromeClosed;
const pending = new Map();
let sequence = 0;
let setupStage = 'chrome_launch';
let setupFailure;
let receipt = { result: 'fail', stage: 'chrome_setup' };
try {
  chrome = spawn(process.env.CHROME_EXECUTABLE || 'google-chrome', [
    '--headless=new', '--no-sandbox', '--disable-dev-shm-usage',
    '--autoplay-policy=no-user-gesture-required', '--remote-debugging-port=0',
    '--remote-debugging-address=127.0.0.1', '--no-first-run',
    '--user-data-dir=' + profile, 'about:blank',
  ], { stdio: 'ignore' });
  chromeClosed = new Promise(resolveClose => chrome.once('close', () => resolveClose(true)));
  let spawnError;
  chrome.on('error', error => {
    spawnError = ['ENOENT', 'EACCES', 'ENOEXEC'].includes(error.code) ? error.code : 'other';
  });
  const deadline = Date.now() + 180_000;
  let port;
  setupStage = 'devtools_port';
  while (!port) {
    if (spawnError) { setupFailure = `spawn_${spawnError}`; throw new Error('chrome_setup'); }
    if (chrome.exitCode !== null) { setupFailure = 'early_exit'; throw new Error('chrome_setup'); }
    if (Date.now() > deadline) { setupFailure = 'devtools_port_timeout'; throw new Error('chrome_setup'); }
    try { port = Number((await readFile(join(profile, 'DevToolsActivePort'), 'utf8')).split('\n')[0]); }
    catch { await new Promise(r => setTimeout(r, 100)); }
  }
  setupStage = 'devtools_page_create';
  const page = await (await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, { method: 'PUT', signal: AbortSignal.timeout(5000) })).json();
  setupStage = 'cdp_websocket';
  socket = new WebSocket(page.webSocketDebuggerUrl);
  await new Promise((resolveOpen, reject) => { socket.addEventListener('open', resolveOpen, { once: true }); socket.addEventListener('error', reject, { once: true }); });
  socket.addEventListener('message', event => {
    const message = JSON.parse(event.data);
    const waiter = pending.get(message.id);
    if (!waiter) return;
    pending.delete(message.id);
    clearTimeout(waiter.timer);
    message.error ? waiter.reject(new Error('cdp_command')) : waiter.resolve(message.result);
  });
  function command(method, params = {}) {
    return new Promise((resolveCommand, reject) => {
      const id = ++sequence;
      const timer = setTimeout(() => { pending.delete(id); reject(new Error('cdp_timeout')); }, 5000);
      pending.set(id, { resolve: resolveCommand, reject, timer });
      socket.send(JSON.stringify({ id, method, params }));
    });
  }
  // Keep this a local fixture: external decoder/CDN fallback is a failure.
  setupStage = 'cdp_fetch_enable';
  await command('Fetch.enable', { patterns: [{ urlPattern: '*' }] });
  const origin = `http://127.0.0.1:${server.address().port}`;
  socket.addEventListener('message', event => {
    const message = JSON.parse(event.data);
    if (message.method !== 'Fetch.requestPaused') return;
    const request = message.params;
    const allowed = request.request.url.startsWith(origin + '/');
    command(allowed ? 'Fetch.continueRequest' : 'Fetch.failRequest', allowed
      ? { requestId: request.requestId }
      : { requestId: request.requestId, errorReason: 'BlockedByClient' }).catch(() => {});
  });
  setupStage = 'page_navigation';
  await command('Page.navigate', { url: origin + '/index.html' });
  receipt = { result: 'fail', stage: 'decoder_timeout' };
  while (Date.now() < deadline) {
    const value = await command('Runtime.evaluate', { expression: 'document.documentElement?.getAttribute("data-music-result")', returnByValue: true });
    if (value.result?.value) { receipt = JSON.parse(value.result.value); break; }
    await new Promise(r => setTimeout(r, 200));
  }
  const expected = ['track.mp3', 'track.flac', 'vorbis.ogg', 'opus.ogg', 'flac.ogg', 'pcm.wav', 'float.wav']
    .flatMap(fixture => [false, true].map(encrypted => ({ fixture, encrypted, result: 'pass' })));
  if (receipt.result !== 'pass' || JSON.stringify(receipt.cases) !== JSON.stringify(expected)) process.exitCode = 1;
} catch {
  process.exitCode = 1;
  if (receipt.stage === 'chrome_setup') {
    receipt = { result: 'fail', stage: setupStage, reason: setupFailure || 'setup_operation_failed' };
    if (Number.isInteger(chrome?.exitCode)) receipt.exit_code = chrome.exitCode;
    if (chrome?.signalCode) receipt.signal = chrome.signalCode;
  }
} finally {
  for (const waiter of pending.values()) clearTimeout(waiter.timer);
  const cleanupFailures = [];
  const waitForChromeClose = async timeoutMs => {
    if (!chromeClosed) return true;
    let timer;
    const didClose = await Promise.race([
      chromeClosed,
      new Promise(resolveClose => { timer = setTimeout(() => resolveClose(false), timeoutMs); }),
    ]);
    clearTimeout(timer);
    return didClose;
  };
  if (chrome?.pid) {
    if (chrome.exitCode === null && socket?.readyState === WebSocket.OPEN) {
      try { socket.send(JSON.stringify({ id: ++sequence, method: 'Browser.close' })); } catch {}
    }
    let closed = await waitForChromeClose(5000);
    if (!closed && chrome.exitCode === null) {
      chrome.kill('SIGTERM');
      closed = await waitForChromeClose(2000);
    }
    if (!closed && chrome.exitCode === null) {
      chrome.kill('SIGKILL');
      closed = await waitForChromeClose(5000);
    }
    if (!closed) cleanupFailures.push('chrome_shutdown_timeout');
  }
  socket?.close();
  server.closeAllConnections();
  await new Promise(r => server.close(r));
  try {
    await rm(profile, { recursive: true, force: true });
  } catch {
    cleanupFailures.push('profile_remove_failed');
  }
  if (cleanupFailures.length) {
    receipt.cleanup = cleanupFailures;
    process.exitCode = 1;
  }
  await writeFile(join(root, 'music-playback-result.json'), JSON.stringify(receipt) + '\n');
  console.log(JSON.stringify(receipt));
}
