// Built Admin in a real Chromium browser; only the HTTP backend is a fixture.
// Node 24 built-ins only. Never use this fixture as live authority evidence.
import assert from 'node:assert/strict';
import { spawn, spawnSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdir, mkdtemp, readFile, readdir, writeFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { extname, join, resolve, sep } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { fileURLToPath } from 'node:url';
import { bodyTextIncludesExpression } from './text-readiness.mjs';

const CHROMIUM_STARTUP_TIMEOUT_MS = 20000;
const CHROMIUM_STARTUP_FAILURE = 'Chromium startup did not become ready';
const SAFE_SIGNALS = new Set(['SIGABRT', 'SIGBUS', 'SIGFPE', 'SIGILL', 'SIGINT', 'SIGKILL', 'SIGSEGV', 'SIGTERM']);

export function sanitizeBrowserVersion(output) {
  const match = /^(?:Google Chrome|Chromium) (\d{1,4}\.\d{1,4}\.\d{1,6}\.\d{1,4})(?:\s|$)/.exec(output);
  return match?.[1] ?? 'unknown';
}

export async function browserVersion(binary, spawnProcess = spawn, timeoutMs = 1000) {
  return new Promise(resolveVersion => {
    let child, output = Buffer.alloc(0), finished = false;
    const finish = value => {
      if (finished) return;
      finished = true;
      clearTimeout(timer);
      child?.stdout?.destroy();
      child?.unref();
      resolveVersion(value);
    };
    const timer = setTimeout(() => {
      try { child?.kill('SIGKILL'); } catch { /* diagnostic only */ }
      finish('unknown');
    }, timeoutMs);
    try {
      child = spawnProcess(binary, ['--version'], { stdio: ['ignore', 'pipe', 'ignore'] });
      child.stdout.on('data', chunk => {
        const bytes = Buffer.from(chunk);
        if (output.length + bytes.length > 1024) {
          try { child.kill('SIGKILL'); } catch { /* diagnostic only */ }
          finish('unknown');
        } else output = Buffer.concat([output, bytes]);
      });
      child.on('error', () => finish('unknown'));
      child.on('close', code => finish(code === 0 ? sanitizeBrowserVersion(output.toString('utf8')) : 'unknown'));
    } catch { finish('unknown'); }
  });
}

function safeSpawnError(error) {
  if (error?.code === 'ENOENT') return 'not_found';
  if (error?.code === 'EACCES' || error?.code === 'EPERM') return 'permission_denied';
  if (['EAGAIN', 'EMFILE', 'ENFILE'].includes(error?.code)) return 'resource_exhausted';
  return 'unknown';
}

function safeSignal(signal) {
  if (signal === 'unknown') return 'unknown';
  if (signal === null) return 'none';
  if (typeof signal !== 'string') return 'unknown';
  return SAFE_SIGNALS.has(signal) ? signal : 'other';
}

function safeExit(code) {
  if (code === 0) return 'zero';
  if (Number.isInteger(code)) return 'nonzero';
  return 'unknown';
}

function startupDiagnostic(state, startedAt, now, { code = null, signal = null, spawnError = 'none' } = {}) {
  return {
    state,
    elapsedMs: Math.max(0, Math.round(now() - startedAt)),
    exit: safeExit(code),
    signal: safeSignal(signal),
    spawnError,
  };
}

export async function launchChromium(binary, args, profile, dependencies = {}) {
  const now = dependencies.now ?? Date.now;
  const sleep = dependencies.sleep ?? delay;
  const spawnProcess = dependencies.spawnProcess ?? spawn;
  const readPort = dependencies.readPort ?? (async () => (await readFile(join(profile, 'DevToolsActivePort'), 'utf8')).split('\n')[0] || null);
  const startedAt = now();
  let child;
  try {
    child = spawnProcess(binary, args, { stdio: ['ignore', 'ignore', 'pipe'] });
  } catch (error) {
    return { child: undefined, port: undefined, diagnostic: startupDiagnostic(
      'spawn_failed', startedAt, now, { signal: 'unknown', spawnError: safeSpawnError(error) }) };
  }

  let spawnError = 'none';
  let portRead = 'missing', stderr = Buffer.alloc(0), stderrTruncated = false;
  child.stderr?.on('data', chunk => {
    const bytes = Buffer.from(chunk);
    const available = 8192 - stderr.length;
    if (bytes.length > available) stderrTruncated = true;
    if (available > 0) stderr = Buffer.concat([stderr, bytes.subarray(0, available)]);
  });
  const diagnostic = result => {
    const stderrSignatures = [
    ['dbus', /(?:dbus\/bus\.cc|dbus\/object_proxy\.cc)/i],
    ['sandbox', /(?:no usable sandbox|sandbox.*(?:failed|error))/i],
    ['resource_exhaustion', /(?:out of memory|resource temporarily unavailable)/i],
    ['devtools_listening', /DevTools listening on /],
    ].filter(([, pattern]) => pattern.test(stderr.toString('utf8'))).map(([name]) => name);
    child.stderr?.destroy();
    return { ...result, portRead, stderrSignatures, stderrTruncated };
  };
  let didExit = false;
  let exitCode = null;
  let signalCode = null;
  child.on('error', error => { spawnError = safeSpawnError(error); });
  child.on('exit', (code, signal) => { didExit = true; exitCode = code; signalCode = signal; });

  const failed = () => {
    if (spawnError !== 'none') {
      return startupDiagnostic('spawn_failed', startedAt, now, { signal: 'unknown', spawnError });
    }
    const code = child.exitCode ?? exitCode;
    const signal = child.signalCode ?? signalCode;
    if (didExit || code !== null && code !== undefined || signal !== null && signal !== undefined) {
      return startupDiagnostic('early_exit', startedAt, now, { code, signal });
    }
    return null;
  };

  const deadline = startedAt + CHROMIUM_STARTUP_TIMEOUT_MS;
  while (now() < deadline) {
    const failure = failed();
    if (failure) return { child, port: undefined, diagnostic: diagnostic(failure) };
    let port;
    try {
      port = await readPort();
      if (port && (!/^\d{1,5}$/.test(port) || Number(port) < 1 || Number(port) > 65535)) { portRead = 'invalid'; port = null; }
    } catch (error) {
      if (error.code === 'EACCES' || error.code === 'EPERM') portRead = 'permission_denied';
      else if (error.code !== 'ENOENT' && portRead !== 'permission_denied') portRead = 'other';
    }
    const afterReadFailure = failed();
    if (afterReadFailure) return { child, port: undefined, diagnostic: diagnostic(afterReadFailure) };
    if (port) return { child, port, diagnostic: diagnostic(startupDiagnostic('ready', startedAt, now)) };
    await sleep(Math.min(100, Math.max(0, deadline - now())));
  }
  const failure = failed();
  return {
    child,
    port: undefined,
    diagnostic: diagnostic(failure ?? startupDiagnostic('port_missing', startedAt, now)),
  };
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
const root = resolve('dist');
const output = resolve(process.env.ADMIN_BROWSER_OUTPUT ?? '../../tmp/admin-browser');
await mkdir(output, { recursive: true });
const evidence = { passed: false, sourceSha: process.env.GITHUB_SHA, checks: [], requests: [] };
assert.match(evidence.sourceSha ?? '', /^[a-f0-9]{40}$/, 'GITHUB_SHA must bind tested source');
const staffId = '11111111-1111-4111-8111-111111111111';
const targetId = '22222222-2222-4222-8222-222222222222';
const accountId = '33333333-3333-4333-8333-333333333333';
// Deliberately synthetic, unsigned token: accepted only by this loopback fixture.
const token = `fixture.${Buffer.from(JSON.stringify({ profile_id: staffId })).toString('base64url')}.fixture`;
const report = { id: 'report-browser-1', reporter_profile_id: 'reporter', target_type: 'user',
  target_id: targetId, category: 'spam', description: 'Browser report', evidence_json: '{}',
  resolution_json: '{}', status: 'pending', created_at: '2026-01-01T00:00:00Z' };
const appeal = { id: 'appeal-browser-1', sanction_id: 'sanction-browser-1',
  appellant_account_id: accountId, reason: 'Browser appeal', status: 'pending', created_at: report.created_at };
let authorization;
let tokenExchanges = 0;
let denyResolve = true;
let sanctionCount = 0;
let fixtureError;
let base;
const json = (res, body, status = 200) => {
  res.writeHead(status, { 'content-type': 'application/json', 'cache-control': 'no-store' });
  res.end(JSON.stringify(body));
};
const server = createServer(async (req, res) => {
  try {
    const url = new URL(req.url, base);
    const path = url.pathname;
    if (path.startsWith('/api/')) {
      evidence.requests.push({ method: req.method, path: `${path}${url.search}` });
      if (path === '/api/v1/auth/oauth2/authorize') {
        assert.equal(url.searchParams.get('client_id'), 'voice-admin');
        assert.equal(url.searchParams.get('response_type'), 'code');
        assert.equal(url.searchParams.get('code_challenge_method'), 'S256');
        assert.equal(url.searchParams.get('redirect_uri'), `${base}/callback`);
        assert.ok(url.searchParams.get('state'));
        assert.match(url.searchParams.get('code_challenge'), /^[\w-]{43}$/);
        authorization = url.searchParams;
        res.writeHead(302, { location: `${base}/callback?code=fixture-code&state=${authorization.get('state')}` }).end();
        return;
      }
      let raw = '';
      for await (const chunk of req) raw += chunk;
      if (path === '/api/v1/auth/oauth2/token') {
        const form = new URLSearchParams(raw);
        assert.equal(req.method, 'POST');
        assert.equal(form.get('grant_type'), 'authorization_code');
        assert.equal(form.get('code'), 'fixture-code');
        assert.equal(form.get('client_id'), 'voice-admin');
        assert.equal(form.get('redirect_uri'), `${base}/callback`);
        assert.equal(createHash('sha256').update(form.get('code_verifier')).digest('base64url'), authorization.get('code_challenge'));
        tokenExchanges++;
        json(res, { access_token: token, token_type: 'Bearer', expires_in: 3600 });
        return;
      }
      assert.equal(req.headers.authorization, `Bearer ${token}`, 'Every staff request uses the issued session');
      const body = raw ? JSON.parse(raw) : undefined;
      if (path === '/api/v1/admin/moderation/reports' && req.method === 'GET') {
        assert.ok(['content', 'spaces'].includes(url.searchParams.get('queue')));
        const status = url.searchParams.get('status');
        const rows = url.searchParams.get('queue') === 'content' && (!status || status === report.status) ? [report] : [];
        json(res, { report_list: { reports: rows } });
      } else if (path === `/api/v1/users/profiles/${targetId}`) {
        json(res, { profile: { account_id: accountId } });
      } else if (path === '/api/v1/admin/moderation/sanctions' && req.method === 'GET') {
        assert.equal(url.searchParams.get('account_id'), accountId);
        json(res, { sanction_list: { sanctions: [] } });
      } else if (path === `/api/v1/admin/moderation/reports/${report.id}/resolve` && req.method === 'POST') {
        if (body.new_status === 'reviewing') {
          assert.deepEqual(body, { new_status: 'reviewing', assigned_to_profile_id: staffId, resolution_json: '{}' });
          report.assigned_to_profile_id = staffId;
        } else {
          assert.equal(body.new_status, 'resolved');
          assert.deepEqual(JSON.parse(body.resolution_json), { note: 'Browser resolution' });
          if (denyResolve) { denyResolve = false; json(res, { error: 'fixture_resolve_denied' }, 403); return; }
        }
        report.status = body.new_status;
        json(res, { report });
      } else if (path === '/api/v1/admin/moderation/sanctions' && req.method === 'POST') {
        assert.deepEqual(body, { target_account_id: accountId, type: 'perm_ban', reason: `Sanction from report ${report.id}`, report_id: report.id });
        sanctionCount++;
        json(res, { sanction: { id: appeal.sanction_id } });
      } else if (path === '/api/v1/admin/moderation/appeals' && req.method === 'GET') {
        json(res, { appeal_list: { appeals: appeal.status === 'pending' ? [appeal] : [] } });
      } else if (path === `/api/v1/admin/moderation/appeals/${appeal.id}/review` && req.method === 'POST') {
        assert.deepEqual(body, { status: 'approved', moderator_note: 'Browser appeal review' });
        appeal.status = body.status;
        json(res, { appeal });
      } else throw new Error(`Unexpected fixture request ${req.method} ${req.url}`);
      return;
    }
    const asset = path.startsWith('/assets/');
    const file = resolve(root, asset ? `.${path}` : 'index.html');
    assert.ok(file.startsWith(`${root}${sep}`));
    res.writeHead(200, { 'content-type': ({ '.js': 'text/javascript', '.css': 'text/css', '.html': 'text/html' })[extname(file)] ?? 'application/octet-stream' });
    res.end(await readFile(file));
  } catch (err) {
    fixtureError = err;
    json(res, { error: String(err) }, 500);
  }
});

async function until(label, check) {
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline) {
    if (fixtureError) throw fixtureError;
    const value = await check();
    if (value) return value;
    await delay(100);
  }
  throw new Error(`Timed out: ${label}`);
}
class Cdp {
  constructor(socket) {
    this.socket = socket; this.id = 0; this.pending = new Map(); this.exceptions = [];
    socket.addEventListener('message', ({ data }) => {
      const message = JSON.parse(data);
      if (message.method === 'Runtime.exceptionThrown') this.exceptions.push(message.params);
      const entry = this.pending.get(message.id);
      if (!entry) return;
      this.pending.delete(message.id); clearTimeout(entry.timer);
      if (message.error) entry.reject(new Error(JSON.stringify(message.error)));
      else entry.resolve(message.result);
    });
  }
  send(method, params = {}) {
    const id = ++this.id;
    return new Promise((resolveMessage, reject) => {
      const timer = setTimeout(() => { this.pending.delete(id); reject(new Error(`CDP timeout ${method}`)); }, 30000);
      this.pending.set(id, { resolve: resolveMessage, reject, timer });
      const sessionId = /^(Target|Browser)\./.test(method) ? undefined : this.sessionId;
      this.socket.send(JSON.stringify({ id, method, params, sessionId }));
    });
  }
}
let chrome;
let cdp;
let screenshot;
try {
  await new Promise(done => server.listen(0, '127.0.0.1', done));
  base = `http://127.0.0.1:${server.address().port}`;
  const build = spawnSync(process.execPath, ['browser/build.mjs', base], { stdio: 'inherit' });
  if (build.error) throw build.error;
  assert.equal(build.status, 0, 'Build actual Admin against reserved fixture origin');
  evidence.bundle = [];
  for (const file of ['index.html', ...(await readdir(join(root, 'assets'))).map(file => `assets/${file}`)]) {
    evidence.bundle.push({ file, sha256: createHash('sha256').update(await readFile(join(root, file))).digest('hex') });
  }
  const profile = await mkdtemp(join(tmpdir(), 'voice-admin-browser-'));
  const startup = await launchChromium(process.env.CHROME_BIN ?? 'google-chrome', ['--headless=new', '--no-sandbox',
    '--disable-dev-shm-usage', '--no-first-run', '--no-default-browser-check', '--remote-debugging-port=0',
    '--disable-extensions', '--disable-background-networking', '--disable-sync', '--no-proxy-server',
    `--user-data-dir=${profile}`, 'about:blank'], profile);
  chrome = startup.child;
  evidence.browserStartup = startup.diagnostic;
  if (startup.diagnostic.state !== 'ready') {
    evidence.browserStartup.version = await browserVersion(process.env.CHROME_BIN ?? 'google-chrome');
    throw new Error(CHROMIUM_STARTUP_FAILURE);
  }
  const port = startup.port;
  evidence.browser = await (await fetch(`http://127.0.0.1:${port}/json/version`)).json();
  const socket = new WebSocket(evidence.browser.webSocketDebuggerUrl);
  await new Promise((done, reject) => { socket.addEventListener('open', done, { once: true }); socket.addEventListener('error', reject, { once: true }); });
  cdp = new Cdp(socket);
  const { targetId } = await cdp.send('Target.createTarget', { url: 'about:blank' });
  const { sessionId } = await cdp.send('Target.attachToTarget', { targetId, flatten: true });
  cdp.sessionId = sessionId;
  const evaluate = async expression => {
    const result = await cdp.send('Runtime.evaluate', { expression, returnByValue: true });
    assert.equal(result.exceptionDetails, undefined);
    return result.result.value;
  };
  const locate = selector => evaluate(`(() => { const e=document.querySelector(${JSON.stringify(selector)}); if(!e || e.disabled) return null; e.scrollIntoView({block:'center'}); const r=e.getBoundingClientRect(); return r.width&&r.height?{x:r.x+r.width/2,y:r.y+r.height/2}:null; })()`);
  const click = async selector => {
    const point = await until(selector, () => locate(selector));
    for (const type of ['mousePressed', 'mouseReleased']) await cdp.send('Input.dispatchMouseEvent', { type, ...point, button: 'left', clickCount: 1 });
  };
  const fill = async (selector, text) => { await click(selector); await cdp.send('Input.insertText', { text }); };
  const text = value => until(value, () => evaluate(bodyTextIncludesExpression(value)));
  const absent = selector => until(`absent ${selector}`, () => evaluate(`!document.querySelector(${JSON.stringify(selector)})`));
  const check = name => evidence.checks.push(name);
  screenshot = async name => { const { data } = await cdp.send('Page.captureScreenshot', { format: 'png' }); await writeFile(join(output, `${name}.png`), Buffer.from(data, 'base64')); };
  await cdp.send('Page.enable'); await cdp.send('Runtime.enable');
  await cdp.send('Emulation.setDeviceMetricsOverride', { width: 1440, height: 1000, deviceScaleFactor: 1, mobile: false });
  await cdp.send('Page.navigate', { url: `${base}/queue` });
  await text('Sign in with your Voice staff account.');
  assert.equal(evidence.requests.length, 0, 'Anonymous entry must not request staff data');
  check('anonymous route gate');
  await cdp.send('Page.navigate', { url: `${base}/callback?code=untrusted&state=wrong` });
  await text('invalid_state'); assert.equal(tokenExchanges, 0);
  check('invalid OAuth state blocks exchange');
  await cdp.send('Page.navigate', { url: base });
  await text('Sign in with your Voice staff account.'); await click('main button');
  await until('OAuth queue', () => locate('[data-testid="report-row-report-browser-1"]'));
  assert.equal(tokenExchanges, 1); check('real OAuth redirect and S256 exchange');
  await click('[data-testid="queue-tab-spaces"]'); await text('No reports in this queue.');
  await click('[data-testid="queue-tab-content"]'); await click('[data-testid="report-row-report-browser-1"]');
  await click('[data-testid="assign-to-me"]'); await until('assigned staff', () => evaluate(`document.querySelector('[data-testid="report-detail"]').innerText.includes('${staffId}')`));
  check('separate queue and exact staff assignment');
  await fill('[data-testid="resolution-note"]', 'Browser resolution');
  await click('[data-testid="resolve-report"]'); await text('fixture_resolve_denied');
  assert.equal(report.status, 'reviewing');
  assert.equal(await evaluate(`document.querySelector('[data-testid="resolution-note"]').value`), 'Browser resolution');
  check('denied resolution preserves report and note'); await screenshot('denied-resolution');
  await click('[data-testid="sanction-perm_ban"]'); await text('Apply destructive sanction?');
  assert.equal(sanctionCount, 0); await click('[role="dialog"] .btn-ghost');
  await absent('[role="dialog"]'); assert.equal(sanctionCount, 0);
  await click('[data-testid="sanction-perm_ban"]'); await click('[data-testid="confirm-dialog-confirm"]');
  await absent('[role="dialog"]'); assert.equal(sanctionCount, 1);
  check('destructive sanction Cancel sends nothing and Confirm sends exact target once');
  await click('[data-testid="resolve-report"]'); await text('Select a report to view details.');
  assert.equal(report.status, 'resolved'); check('successful retry closes report');
  await click('a[href="/appeals"]'); await click('[data-testid="appeal-row-appeal-browser-1"]');
  await fill('[data-testid="appeal-moderator-note"]', 'Browser appeal review');
  await click('[data-testid="approve-appeal"]'); await text('No appeals in this queue.');
  assert.equal(appeal.status, 'approved'); check('appeal review preserves exact sanction-linked appeal');
  await screenshot('appeal-reviewed');
  await click('.nav-signout'); await text('Sign in with your Voice staff account.');
  assert.equal(await evaluate(`sessionStorage.getItem('voice_admin_access_token')`), null);
  const count = evidence.requests.length;
  await cdp.send('Page.navigate', { url: `${base}/queue` }); await text('Sign in with your Voice staff account.');
  assert.equal(evidence.requests.length, count); check('logout clears session and gates direct reload');
  assert.equal(cdp.exceptions.length, 0); assert.equal(fixtureError, undefined);
  evidence.passed = true;
  console.log(`PASS ${evidence.checks.length} Admin browser checks (${evidence.sourceSha})`);
} catch (err) {
  evidence.error = err.message === CHROMIUM_STARTUP_FAILURE ? err.message : err.stack ?? String(err);
  process.exitCode = 1; console.error(evidence.error);
  try { await screenshot?.('failure'); } catch { /* preserve original failure */ }
} finally {
  await writeFile(join(output, 'evidence.json'), `${JSON.stringify(evidence, null, 2)}\n`);
  if (cdp) {
    try { await cdp.send('Browser.close'); } catch { /* disconnected while closing */ }
    cdp.socket.close();
  }
  if (chrome && chrome.exitCode === null && chrome.pid) chrome.kill();
  server.closeAllConnections(); await new Promise(done => server.close(done));
}
}
