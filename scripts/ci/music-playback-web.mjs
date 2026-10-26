// Hosted Chrome integration: real package WASM/worker and application decoder.
import { createServer } from 'node:http';
import { spawn } from 'node:child_process';
import { mkdtemp, readFile, realpath, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { resolve, sep, extname, join } from 'node:path';
import { once } from 'node:events';

const root = await realpath(process.argv[2]);
const profile = await mkdtemp(join(tmpdir(), 'voice-music-chrome-'));
// Observability only: never use diagnostic text to decide the probe result.
function diagnosticText(value, maxBytes) {
  const text = String(value)
    .replace(/\x1b\[[0-?]*[ -/]*[@-~]/g, '')
    .replace(/[\u0000-\u001f\u007f]/g, ' ')
    .replaceAll(profile, '<chrome-profile>')
    .replace(/\bBearer\s+[^\s"']+/gi, 'Bearer <redacted>')
    .replace(/(["']?(?:authorization|cookie|set-cookie|password|secret|(?:access_|refresh_)?token|api[_-]?key)["']?\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;}]+)/gi, '$1<redacted>');
  let result = '', bytes = 0;
  for (const character of text) {
    const size = Buffer.byteLength(character);
    if (bytes + size > maxBytes) break;
    result += character;
    bytes += size;
  }
  return result;
}
const diagnostics = { node_version: process.version };
let chromeStderr = '', chromeStderrTruncated = false;
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
  ], { stdio: ['ignore', 'ignore', 'pipe'] });
  chrome.stderr.setEncoding('utf8');
  chrome.stderr.on('data', chunk => {
    const text = chromeStderr + chunk;
    chromeStderrTruncated ||= Buffer.byteLength(text) > 8192;
    chromeStderr = diagnosticText(text, 8192);
  });
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
  const response = await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, { method: 'PUT', signal: AbortSignal.timeout(5000) });
  diagnostics.page_http_status = response.status;
  const pageBody = await response.text();
  diagnostics.page_body = diagnosticText(pageBody, 2048);
  diagnostics.page_body_truncated = Buffer.byteLength(pageBody) > 2048;
  const page = JSON.parse(pageBody);
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
} catch (error) {
  diagnostics.stage = setupStage;
  diagnostics.error_name = ['AbortError', 'TimeoutError', 'SyntaxError', 'TypeError', 'Error'].includes(error?.name) ? error.name : 'Other';
  diagnostics.error_category = ['AbortError', 'TimeoutError'].includes(error?.name)
    ? 'timeout_or_abort' : error?.name === 'SyntaxError' ? 'json_parse' : 'setup_operation';
  const code = error?.cause?.code ?? error?.code;
  if (typeof code === 'string' && /^[A-Z][A-Z0-9_]{0,63}$/.test(code)) diagnostics.error_code = code;
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
  diagnostics.chrome_stderr = diagnosticText(chromeStderr, 8192);
  diagnostics.chrome_stderr_truncated = chromeStderrTruncated;
  console.error(JSON.stringify({ music_playback_diagnostics: diagnostics }));
  try {
    await writeFile(join(root, 'music-playback-diagnostics.json'), JSON.stringify(diagnostics) + '\n');
  } catch {
    console.error('Music diagnostic artifact could not be written');
  }
  console.log(JSON.stringify(receipt));
}
