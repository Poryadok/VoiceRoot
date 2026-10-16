import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import vm from 'node:vm';
import test from 'node:test';

const html = readFileSync(new URL('../../src/frontend/web/index.html', import.meta.url), 'utf8');
const scripts = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)];
const initializers = scripts.filter((entry) => entry[1].includes('voiceMusicReady'));
assert.equal(initializers.length, 1);
assert.ok(html.indexOf('libflutter_soloud_plugin.js') < html.indexOf('voiceMusicReady'));
assert.ok(html.indexOf('voiceMusicReady') < html.indexOf('<script src="flutter_bootstrap.js"'));

test('readiness waits for the instance and initializes exactly once', async () => {
  let resolve;
  let calls = 0;
  const instance = {};
  const pending = new Promise((done) => { resolve = done; });
  const context = vm.createContext({ Module_soloud: () => { calls++; return pending; } });
  vm.runInContext(initializers[0][1], context);
  await Promise.resolve();
  assert.equal(calls, 1);
  assert.equal(typeof context.Module_soloud, 'function');
  resolve(instance);
  await context.voiceMusicReady;
  assert.equal(context.Module_soloud, instance);
  await context.voiceMusicReady;
  assert.equal(calls, 1);
});

test('module failure remains rejected for the Music consumer', async () => {
  const context = vm.createContext({ Module_soloud: () => Promise.reject(new Error('fixture')) });
  vm.runInContext(initializers[0][1], context);
  await assert.rejects(context.voiceMusicReady);
  assert.equal(typeof context.Module_soloud, 'function');
});

test('missing package factory fails closed', async () => {
  const context = vm.createContext({});
  vm.runInContext(initializers[0][1], context);
  await assert.rejects(context.voiceMusicReady);
});
