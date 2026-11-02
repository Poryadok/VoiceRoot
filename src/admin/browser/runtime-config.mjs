// Verify the actual production entry point and hashed bundle in an isolated browser.
// Usage: node src/admin/browser/runtime-config.mjs <dist directory> <admin|portal>
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { createServer } from 'node:http';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { extname, join, resolve, sep } from 'node:path';
import { startChrome } from './runtime-config-startup.mjs';

const root = resolve(process.argv[2]);
const app = process.argv[3];
assert.ok(['admin', 'portal'].includes(app));
const index = await readFile(join(root, 'index.html'));
const asset = /src="(\/assets\/index-[^"]+\.js)"/.exec(index.toString())?.[1];
assert.ok(asset, 'production HTML must reference a hashed entry bundle');
const assetBytes = await readFile(join(root, asset.slice(1)));
const assetSha = createHash('sha256').update(assetBytes).digest('hex');
const sleep = ms => new Promise(done => setTimeout(done, ms));
const servers = [];
const startServer = async handler => {
  const server = createServer(handler);
  await new Promise(done => server.listen(0, '127.0.0.1', done));
  servers.push(server);
  return `http://127.0.0.1:${server.address().port}`;
};
const apiCalls = [];
const serviceCalls = () => apiCalls.filter(call => call.path.startsWith('/api/'));
const apis = await Promise.all(['dev', 'stage'].map(env => startServer((req, res) => {
  apiCalls.push({ env, path: req.url });
  res.writeHead(200, { 'Content-Type': 'text/plain', 'Access-Control-Allow-Origin': '*' });
  res.end('fixture');
})));
const cases = [];
const webs = await Promise.all(['dev', 'stage'].map(async (env, number) => {
  const state = { config: null, status: 200, requests: 0, delivered: false };
  cases.push(state);
  const origin = await startServer(async (req, res) => {
    if (req.url === '/voice-runtime-config.json') {
      state.requests++;
      await sleep(100);
      state.delivered = true;
      res.writeHead(state.status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' });
      res.end(state.config === null ? '' : JSON.stringify(state.config));
      return;
    }
    try {
      const name = new URL(req.url, 'http://fixture').pathname;
      const file = resolve(root, '.' + (name === '/' ? '/index.html' : name));
      assert.ok(file.startsWith(root + sep));
      const bytes = await readFile(file);
      const mime = { '.html': 'text/html', '.js': 'application/javascript', '.css': 'text/css' };
      res.writeHead(200, { 'Content-Type': mime[extname(file)] ?? 'application/octet-stream' });
      res.end(bytes);
    } catch { res.writeHead(404); res.end(); }
  });
  state.config = { version: 1, app_origin: origin, api_base_url: apis[number] };
  return origin;
}));

const profile = await mkdtemp(join(tmpdir(), `voice-${app}-config-`));
const chrome = startChrome(process.env.CHROME_BIN || (process.platform === 'win32'
  ? 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe' : 'google-chrome'), [
  '--headless=new', '--no-sandbox', '--disable-dev-shm-usage', '--no-first-run',
  '--remote-debugging-port=0', `--user-data-dir=${profile}`, 'about:blank',
], profile);
let socket;
let startupFailed = true;
try {
  const port = await chrome.port();
  assert.ok(port, 'Chrome did not publish a DevTools port');
  startupFailed = false;
  const pages = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
  const page = pages.find(row => row.type === 'page');
  assert.ok(page);
  socket = new WebSocket(page.webSocketDebuggerUrl);
  await new Promise(done => socket.addEventListener('open', done, { once: true }));
  let id = 0;
  const pending = new Map();
  const networkApiCalls = [];
  const externalRequests = [];
  socket.addEventListener('message', event => {
    const reply = JSON.parse(event.data);
    if (reply.method === 'Network.requestWillBeSent') {
      const url = new URL(reply.params.request.url);
      if (url.pathname.startsWith('/api/')) networkApiCalls.push(url.href);
      if (url.protocol.startsWith('http') && url.hostname !== '127.0.0.1') externalRequests.push(url.href);
    }
    if (!reply.id) return;
    const waiter = pending.get(reply.id);
    if (!waiter) return;
    pending.delete(reply.id);
    if (reply.error) waiter.reject(reply.error);
    else waiter.resolve(reply.result);
  });
  const send = (method, params = {}) => new Promise((resolve, reject) => {
    const key = ++id;
    const timer = setTimeout(() => { pending.delete(key); reject(new Error(`CDP timeout: ${method}`)); }, 10000);
    pending.set(key, {
      resolve: value => { clearTimeout(timer); resolve(value); },
      reject: error => { clearTimeout(timer); reject(error); },
    });
    socket.send(JSON.stringify({ id: key, method, params }));
  });
  await send('Page.enable');
  await send('Runtime.enable');
  await send('Network.enable');
  const evaluate = async expression => (await send('Runtime.evaluate', {
    expression, returnByValue: true, awaitPromise: true,
  })).result.value;
  const until = async predicate => {
    for (let i = 0; i < 100; i++) {
      if (await predicate()) return;
      await sleep(100);
    }
    throw new Error('Browser state timed out');
  };
  const navigate = async url => { await send('Page.navigate', { url }); };
  const loginReady = async () => evaluate(`!![...document.querySelectorAll('button')].find(b => b.textContent.includes('Sign in with Voice'))`);
  const errorReady = async () => evaluate(`!!document.querySelector('[role="alert"] button')`);
  for (let n = 0; n < 2; n++) {
    cases[n].delivered = false;
    await navigate(webs[n]);
    await until(loginReady);
    assert.equal(cases[n].requests, 1);
    assert.ok(cases[n].delivered);
    assert.equal(networkApiCalls.length, n, 'API traffic before explicit login');
    await evaluate(`[...document.querySelectorAll('button')].find(b => b.textContent.includes('Sign in with Voice')).click()`);
    await until(async () => serviceCalls().length === n + 1).catch(async error => {
      throw new Error(`${error.message}; page=${await evaluate('location.href')}; body=${await evaluate('document.body.innerText')}; calls=${JSON.stringify(apiCalls)}`);
    });
    assert.equal(serviceCalls()[n].env, n === 0 ? 'dev' : 'stage');
    assert.match(serviceCalls()[n].path, /^\/api\/v1\/auth\/oauth2\/authorize\?/);
    // The fixture server and DevTools deliver their observations independently.
    await until(() => networkApiCalls.length >= n + 1);
    assert.equal(networkApiCalls.length, n + 1);
  }
  const negatives = [
    ['missing', 404, null],
    ['wrong-origin', 200, { version: 1, app_origin: webs[0], api_base_url: apis[1] }],
    ['invalid', 200, { version: 1, app_origin: webs[1], api_base_url: `${apis[1]}/path` }],
  ];
  for (const [name, status, config] of negatives) {
    cases[1].status = status;
    cases[1].config = config;
    await navigate(webs[1]);
    await until(errorReady);
    assert.equal(networkApiCalls.length, 2, `${name} configuration sent API traffic`);
    assert.equal(await evaluate(`document.querySelector('[role="alert"] button').textContent`), 'Retry loading');
  }
  cases[1].status = 200;
  cases[1].config = { version: 1, app_origin: webs[1], api_base_url: apis[1] };
  await evaluate(`document.querySelector('[role="alert"] button').click()`);
  await until(loginReady);
  assert.equal(networkApiCalls.length, 2, 'retry sent API traffic before config');
  assert.deepEqual(externalRequests, [], 'browser requested a non-fixture origin');
  console.log(JSON.stringify({ app, asset, assetSha, result: 'PASS', cases: ['dev', 'stage', ...negatives.map(x => x[0]), 'retry'] }));
} finally {
  socket?.close();
  try {
    let removeProfile;
    try { removeProfile = await chrome.close(); }
    finally { if (startupFailed) console.error(JSON.stringify({ chrome_startup: chrome.diagnostic() })); }
    if (removeProfile) await rm(profile, { recursive: true, force: true, maxRetries: 10, retryDelay: 200 });
    else console.error('Chrome profile retained: descendant observation unavailable');
  } finally {
    await Promise.all(servers.map(server => new Promise(done => server.close(done))));
  }
}
