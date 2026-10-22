import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';
import test from 'node:test';
import { launchChromium } from './staff-flows.mjs';

function fakeChild() {
  const child = new EventEmitter();
  child.exitCode = null;
  child.signalCode = null;
  child.kill = () => {};
  return child;
}

test('spawn failure records an allowlisted error and redacts message and launch inputs', async () => {
  let clock = 100;
  const child = fakeChild();
  const result = await launchChromium('private-executable', ['private-argument'], 'private-profile', {
    spawnProcess: () => {
      Promise.resolve().then(() => child.emit('error', Object.assign(new Error('private-path-and-token'), { code: 'ENOENT' })));
      return child;
    },
    readPort: async () => null,
    now: () => clock,
    sleep: async milliseconds => { clock += milliseconds; },
  });

  assert.equal(result.diagnostic.state, 'spawn_failed');
  assert.equal(result.diagnostic.spawnError, 'not_found');
  assert.equal(result.diagnostic.exit, 'unknown');
  assert.equal(result.diagnostic.signal, 'unknown');
  assert.deepEqual(Object.keys(result.diagnostic).sort(), ['elapsedMs', 'exit', 'signal', 'spawnError', 'state']);
  assert.equal(JSON.stringify(result).includes('private-'), false);
});

test('early exit records an exit category and signal enum only', async () => {
  const child = fakeChild();
  child.exitCode = 23;
  const result = await launchChromium('unused', [], 'unused', {
    spawnProcess: () => child,
    readPort: async () => null,
    now: () => 500,
    sleep: async () => assert.fail('an exited process should not wait'),
  });

  assert.equal(result.diagnostic.state, 'early_exit');
  assert.equal(result.diagnostic.exit, 'nonzero');
  assert.equal(result.diagnostic.signal, 'none');
  assert.equal(result.diagnostic.spawnError, 'none');
  assert.equal(JSON.stringify(result.diagnostic).includes('23'), false);
});

test('a running process without a port file reaches the unchanged 20-second deadline', async () => {
  let clock = 0;
  const child = fakeChild();
  let launches = 0;
  const result = await launchChromium('unused', [], 'unused', {
    spawnProcess: () => { launches++; return child; },
    readPort: async () => null,
    now: () => clock,
    sleep: async milliseconds => { clock += milliseconds; },
  });

  assert.equal(launches, 1);
  assert.equal(result.diagnostic.state, 'port_missing');
  assert.equal(result.diagnostic.elapsedMs, 20000);
  assert.equal(result.diagnostic.exit, 'unknown');
  assert.equal(result.diagnostic.signal, 'none');
  assert.equal(result.diagnostic.spawnError, 'none');
});
