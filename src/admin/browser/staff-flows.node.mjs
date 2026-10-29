import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';
import { PassThrough } from 'node:stream';
import test from 'node:test';
import * as staff from './staff-flows.mjs';
const { launchChromium } = staff;

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
  assert.deepEqual(Object.keys(result.diagnostic).sort(), ['elapsedMs', 'exit', 'portRead', 'signal', 'spawnError', 'state', 'stderrSignatures', 'stderrTruncated']);
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

test('startup diagnostics report only bounded fixed stderr signatures and port error categories', async () => {
  let clock = 0;
  const child = fakeChild();
  child.stderr = new PassThrough();
  const result = await launchChromium('private-executable', [], 'private-profile', {
    spawnProcess: () => {
      Promise.resolve().then(() => child.stderr.write('dbus/bus.cc private-token https://secret.example ' + 'x'.repeat(9000)));
      return child;
    },
    readPort: async () => { throw Object.assign(new Error('private-path'), { code: 'EACCES' }); },
    now: () => clock,
    sleep: async ms => { clock += ms; await Promise.resolve(); },
  });
  assert.equal(result.diagnostic.state, 'port_missing');
  assert.equal(result.diagnostic.portRead, 'permission_denied');
  assert.deepEqual(result.diagnostic.stderrSignatures, ['dbus']);
  assert.equal(result.diagnostic.stderrTruncated, true);
  assert.doesNotMatch(JSON.stringify(result.diagnostic), /private|secret\.example/);
  assert.equal(child.stderr.destroyed, true, 'an inherited open diagnostic pipe cannot retain the runner');
});

test('version probe returns at deadline even if child never closes', async () => {
  const child = fakeChild();
  child.stdout = new PassThrough();
  let signal, unref = false;
  child.kill = value => { signal = value; };
  child.unref = () => { unref = true; };
  assert.equal(await staff.browserVersion('private', () => child, 10), 'unknown');
  assert.equal(signal, 'SIGKILL');
  assert.equal(child.stdout.destroyed, true);
  assert.equal(unref, true);
});

test('browser version retains only a bounded recognized version', () => {
  assert.equal(staff.sanitizeBrowserVersion?.('Google Chrome 123.4.5678.9\nsecret'), '123.4.5678.9');
  assert.equal(staff.sanitizeBrowserVersion?.('private secret 123.4.5678.9'), 'unknown');
  assert.equal(staff.sanitizeBrowserVersion?.('Google Chrome 123456789.4.5678.9'), 'unknown');
});
