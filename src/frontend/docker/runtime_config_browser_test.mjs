// Run against extracted release image bytes, with Node 24 and Linux Chrome.
// No deployed environment or real API is contacted.
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtemp, readFile, rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join, resolve, extname} from 'node:path';
import {spawn} from 'node:child_process';

const root = resolve(process.argv[2]);
const wait = ms => new Promise(r => setTimeout(r, ms));
const serve = async handler => {
  const server = createServer(handler);
  await new Promise(r => server.listen(0, '127.0.0.1', r));
  return {server, origin: `http://127.0.0.1:${server.address().port}`};
};
const mime = {'.js': 'application/javascript', '.json': 'application/json',
  '.html': 'text/html', '.wasm': 'application/wasm'};
const apiCalls = [];
let configDelivered = false;
const apis = await Promise.all(['dev', 'stage'].map(env => serve((req, res) => {
  apiCalls.push({env, url: req.url, method: req.method, afterConfig: configDelivered});
  res.writeHead(req.method === 'OPTIONS' ? 204 : 401, {'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Headers': '*', 'Access-Control-Allow-Methods': 'POST, GET, OPTIONS',
    'Content-Type': 'application/json'});
  res.end('{"error":"fixture_session_rejected"}');
})));
let config, configStatus = 200, configRequests = 0;
const webHandler = async (req, res) => {
  if (req.url === '/voice-runtime-config.json') {
    configRequests++;
    await wait(150);
    configDelivered = true;
    res.writeHead(configStatus, {'Content-Type': 'application/json', 'Cache-Control': 'no-store'});
    res.end(JSON.stringify(config));
    return;
  }
  try {
    const pathname = new URL(req.url, 'http://fixture').pathname;
    const path = resolve(root, '.' + (pathname === '/' ? '/index.html' : pathname));
    assert.ok(path.startsWith(root + '/'));
    const bytes = await readFile(path.endsWith('/') ? path + 'index.html' : path);
    res.writeHead(200, {'Content-Type': mime[extname(path)] || 'application/octet-stream'});
    res.end(bytes);
  } catch { res.writeHead(404); res.end(); }
};
const webs = await Promise.all([serve(webHandler), serve(webHandler)]);

const profile = await mkdtemp(join(tmpdir(), 'voice-web-config-'));
const chrome = spawn(process.env.CHROME_BIN || 'google-chrome', [
  '--headless', '--no-sandbox', '--disable-dev-shm-usage', '--no-first-run',
  '--remote-debugging-port=0', `--user-data-dir=${profile}`, 'about:blank'],
  {stdio: ['ignore', 'ignore', 'pipe']});
let chromeErrors = '';
chrome.stderr.on('data', b => {chromeErrors += b;});
let socket;
try {
  let port;
  for (let i = 0; i < 100; i++) {
    try { port = +(await readFile(join(profile, 'DevToolsActivePort'), 'utf8')).split('\n')[0]; break; }
    catch {await wait(100);}
  }
  assert.ok(port, chromeErrors);
  const page = (await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()).find(p => p.type === 'page');
  socket = new WebSocket(page.webSocketDebuggerUrl);
  await new Promise(r => socket.addEventListener('open', r, {once: true}));
  const pending = new Map(); let id = 0;
  const events = [];
  socket.addEventListener('message', event => {
    const msg = JSON.parse(event.data);
    if (msg.id) {const p = pending.get(msg.id); pending.delete(msg.id); msg.error ? p.reject(msg.error) : p.resolve(msg.result);}
    else events.push(msg);
  });
  const send = (method, params = {}) => new Promise((resolve, reject) => {
    const key = ++id;
    const timer = setTimeout(() => {pending.delete(key); reject(new Error(`CDP timeout: ${method}`));}, 10000);
    pending.set(key, {resolve: value => {clearTimeout(timer); resolve(value);}, reject: error => {clearTimeout(timer); reject(error);}});
    socket.send(JSON.stringify({id: key, method, params}));
  });
  await send('Page.enable'); await send('Runtime.enable');
  await send('Network.enable');
  // Disposable session forces the production restore path to use its API client.
  // Values are fixtures, never copied from a browser or deployed environment.
  const saved = {access_token: 'fixture-access', refresh_token: 'fixture-refresh',
    account_id: 'fixture-account', profile_id: 'fixture-profile', expires_in_seconds: 60};
  await send('Page.addScriptToEvaluateOnNewDocument', {source:
    `localStorage.setItem('flutter.voice.auth.session', ${JSON.stringify(JSON.stringify(JSON.stringify(saved)))});`});
  // Keep engine resources local as well; no real service is part of this proof.
  await send('Fetch.enable', {patterns: [{urlPattern: '*'}]});
  socket.addEventListener('message', async event => {
    const msg = JSON.parse(event.data);
    if (msg.method !== 'Fetch.requestPaused') return;
    const {requestId, request} = msg.params;
    const url = new URL(request.url);
    try {
    if (url.hostname === '127.0.0.1' || url.protocol === 'data:' || url.protocol === 'blob:') {
      await send('Fetch.continueRequest', {requestId});
    } else if (url.hostname === 'www.gstatic.com' && url.pathname.startsWith('/flutter-canvaskit/')) {
      try {
        const relative = url.pathname.split('/').slice(3).join('/');
        const body = await readFile(join(root, 'canvaskit', relative));
        await send('Fetch.fulfillRequest', {requestId, responseCode: 200,
          responseHeaders: [{name: 'Content-Type', value: mime[extname(relative)] || 'application/octet-stream'},
            {name: 'Access-Control-Allow-Origin', value: '*'}], body: body.toString('base64')});
      } catch {await send('Fetch.failRequest', {requestId, errorReason: 'BlockedByClient'});}
    } else {await send('Fetch.failRequest', {requestId, errorReason: 'BlockedByClient'});}
    } catch (error) {
      // Navigation can cancel a paused request before its response is supplied.
      if (error.code !== -32602 || error.message !== 'Invalid InterceptionId.') throw error;
    }
  });
  for (const [name, env, invalid] of [['dev', 0, false], ['stage', 1, false],
      ['wrong-origin', 0, true], ['missing', 0, true], ['invalid', 0, true]]) {
    const web = webs[env];
    config = {version: 1, app_origin: invalid ? 'https://dev.comrade.click' : web.origin,
      api_base_url: apis[env].origin, livekit_url: 'ws://127.0.0.1:7880'};
    configStatus = name === 'missing' ? 404 : 200;
    if (name === 'invalid') config = {...config, app_origin: web.origin, api_base_url: ''};
    configDelivered = false;
    const before = apiCalls.length, beforeConfig = configRequests;
    events.length = 0;
    await send('Network.clearBrowserCache');
    await send('Page.navigate', {url: web.origin + '/'});
    let rendered = false;
    for (let i = 0; i < 200; i++) {
      const result = await send('Runtime.evaluate', {expression: "document.querySelector('flutter-view') !== null && document.getElementById('voice-startup') === null", returnByValue: true});
      rendered = result.result.value;
      if ((!invalid && rendered && apiCalls.length > before) || (invalid && configRequests > beforeConfig)) break;
      await wait(100);
    }
    if (invalid) {
      await wait(1000);
      const state = await send('Runtime.evaluate', {expression: "document.getElementById('voice-startup') === null", returnByValue: true});
      assert.equal(state.result.value, false); assert.equal(apiCalls.length, before);
      const errorUI = await send('Runtime.evaluate', {expression: `JSON.stringify({
        status: document.getElementById('voice-startup-status').textContent,
        retry: !document.getElementById('voice-startup-retry').hidden,
        progressHidden: document.getElementById('voice-startup-progress').hidden})`, returnByValue: true});
      const errorState = JSON.parse(errorUI.result.value);
      assert.equal(errorState.retry, true);
      assert.equal(errorState.progressHidden, true);
      assert.match(errorState.status, /could not start/);
      console.log(JSON.stringify({case: name, errorState}));
      const requests = events.filter(e => e.method === 'Fetch.requestPaused').map(e => new URL(e.params.request.url));
      assert.ok(requests.every(url => webs.some(w => url.origin === w.origin) ||
        (url.hostname === 'www.gstatic.com' && url.pathname.startsWith('/flutter-canvaskit/')) ||
        (url.hostname === 'fonts.gstatic.com' && url.pathname.startsWith('/s/roboto/') && url.pathname.endsWith('.woff2'))),
      `invalid config attempted an outbound request: ${requests.map(url => url.href).join(', ')}`);
      assert.equal(events.filter(e => e.method === 'Network.webSocketCreated').length, 0);
    }
    else {
      assert.equal(rendered, true, JSON.stringify({configRequests, errors: events.filter(e => e.method === 'Runtime.exceptionThrown')}));
      assert.ok(apiCalls.slice(before).some(c => c.method === 'POST' && c.url.includes('/refresh')), 'real main must restore the fixture session');
      assert.ok(apiCalls.slice(before).every(c => c.afterConfig), 'API used before config delivery');
    }
    assert.equal(configRequests, beforeConfig + 1);
    assert.ok(apiCalls.slice(before).every(c => c.env === name), 'cross-environment request');
    console.log(JSON.stringify({case: name, rendered, apiCalls: apiCalls.slice(before), configRequests: 1}));
    if (name === 'wrong-origin') {
      config = {...config, app_origin: web.origin};
      configDelivered = false;
      await send('Runtime.evaluate', {expression: "document.getElementById('voice-startup-retry').click()"});
      let recovered = false;
      for (let i = 0; i < 200; i++) {
        const result = await send('Runtime.evaluate', {expression: "document.getElementById('voice-startup') === null", returnByValue: true});
        recovered = result.result.value;
        if (recovered && apiCalls.length > before) break;
        await wait(100);
      }
      assert.equal(recovered, true);
      assert.equal(configRequests, beforeConfig + 2);
      assert.ok(apiCalls.slice(before).some(c => c.method === 'POST' && c.url.includes('/refresh')));
      assert.ok(apiCalls.slice(before).every(c => c.env === 'dev' && c.afterConfig));
      console.log(JSON.stringify({case: 'corrected-config-retry', recovered, apiCalls: apiCalls.slice(before)}));
    }
    await send('Page.navigate', {url: 'about:blank'});
    // A navigation acknowledgement precedes the old renderer's final aborts.
    // Drain them before changing the next case's config/order marker.
    await wait(500);
  }
} finally {
  socket?.close();
  if (chrome.exitCode === null && chrome.signalCode === null) {
    chrome.kill('SIGKILL');
    await new Promise(r => chrome.once('close', r));
  }
  for (const s of [...apis, ...webs]) {s.server.closeAllConnections(); await new Promise(r => s.server.close(r));}
  await rm(profile, {recursive: true, force: true});
}
