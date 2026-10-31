import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import vm from 'node:vm';

const html = await readFile(new URL('../../src/frontend/web/index.html', import.meta.url), 'utf8');
function fixture(language = 'ru-RU') {
  const nodes = Object.fromEntries(['voice-startup', 'voice-startup-status', 'voice-startup-retry',
    'voice-startup-progress'].map(id => [id, { hidden: id.endsWith('retry'), textContent: '',
    remove() { this.removed = true; }, addEventListener(type, handler) { this[type] = handler; } }]));
  const events = {};
  const timers = new Map();
  let reloads = 0;
  const window = { addEventListener(type, handler) { events[type] = handler; },
    location: { reload() { reloads++; } } };
  const marks = [];
  const script = html.match(/<script id="voice-startup-controller">([\s\S]*?)<\/script>/)?.[1];
  assert.ok(script, 'startup controller must run before external Flutter resources');
  vm.runInNewContext(script, { window, navigator: { language }, document: {
    documentElement: {}, getElementById: id => nodes[id] },
    performance: { mark: name => marks.push(name) }, setTimeout: (fn, ms) => { timers.set(ms, fn); return ms; },
    clearTimeout: id => timers.delete(id) });
  return { nodes, events, timers, window, marks, reloads: () => reloads };
}

test('logo and status exist before blocking audio and Flutter scripts, with no font dependency', () => {
  assert.ok(html.includes('id="voice-startup"'), 'cold HTML must contain a visible startup panel');
  assert.ok(html.indexOf('id="voice-startup"') < html.indexOf('libflutter_soloud_plugin.js'));
  assert.match(html, /assets\/assets\/app_icons\/voice_sky\.png/);
  assert.match(html, /role="status"/);
  assert.match(html, /prefers-reduced-motion/);
});

test('pre-Flutter copy follows browser RU/EN locale without waiting for app preferences', () => {
  assert.equal(fixture('ru-RU').nodes['voice-startup-status'].textContent, 'Загружаем Voice…');
  const f = fixture('en-GB');
  f.window.voiceStartup.fail();
  assert.equal(f.nodes['voice-startup-retry'].textContent, 'Retry loading');
  assert.match(f.nodes['voice-startup-status'].textContent, /Check your connection/);
});

test('only the first rendered Flutter frame removes the panel; late startup failure cannot replace the app', () => {
  const f = fixture();
  f.window.voiceStartup.stage('engine');
  assert.equal(f.nodes['voice-startup'].removed, undefined);
  f.window.voiceStartup.stage('run-app');
  assert.equal(f.nodes['voice-startup'].removed, undefined);
  f.events['flutter-first-frame']();
  assert.equal(f.nodes['voice-startup'].removed, true);
  assert.equal(f.timers.size, 0);
  f.window.voiceStartup.fail();
  assert.equal(f.nodes['voice-startup-retry'].hidden, true);
  assert.ok(f.marks.includes('voice-startup:first-frame'));
});

test('failed and stalled startup show safe error/retry, and a late first frame recovers', () => {
  for (const fail of [f => f.window.voiceStartup.fail(), f => [...f.timers.values()][0]()]) {
    const f = fixture();
    fail(f);
    assert.equal(f.nodes['voice-startup-retry'].hidden, false);
    assert.equal(f.nodes['voice-startup-progress'].hidden, true);
    assert.ok(f.nodes['voice-startup-status'].textContent.length > 10);
    f.nodes['voice-startup-retry'].click();
    assert.equal(f.reloads(), 1);
    f.events['flutter-first-frame']();
    assert.equal(f.nodes['voice-startup'].removed, true);
  }
});

test('custom bootstrap catches load, engine and app failures and never dismisses on runApp alone', async () => {
  const source = (await readFile(new URL('../../src/frontend/web/flutter_bootstrap.js', import.meta.url), 'utf8'))
    .replaceAll('{{flutter_js}}', '').replaceAll('{{flutter_build_config}}', '');
  for (const failure of ['load', 'engine', 'app', 'none']) {
    const f = fixture();
    const loader = { async load({ onEntrypointLoaded }) {
      if (failure === 'load') throw new Error('private URL');
      await onEntrypointLoaded({ async initializeEngine() {
        if (failure === 'engine') throw new Error('private URL');
        return { async runApp() { if (failure === 'app') throw new Error('private URL'); } };
      } });
    } };
    vm.runInNewContext(source, { window: f.window, _flutter: { loader } });
    await new Promise(resolve => setImmediate(resolve));
    assert.equal(f.nodes['voice-startup'].removed, undefined);
    assert.equal(f.nodes['voice-startup-retry'].hidden, failure === 'none');
    assert.ok(!f.nodes['voice-startup-status'].textContent.includes('private URL'));
  }
});
import './web-startup-poll.test.mjs';
