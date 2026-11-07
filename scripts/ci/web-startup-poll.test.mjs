import assert from 'node:assert/strict';
import { test } from 'node:test';
import { pollAfterNavigation, clickRetry } from './web-startup-poll.mjs';

test('retries only the expected navigation interruption and still waits for a rendered frame', async () => {
  let calls = 0;
  const result = await pollAfterNavigation(async () => {
    calls++;
    if (calls === 1) throw Object.assign(new Error('Inspected target navigated or closed'), { code: -32000 });
    return calls === 3;
  }, { interval: 0, timeout: 1000 });
  assert.equal(result, true);
  assert.equal(calls, 3);
});

test('propagates unrelated errors and times out if the frame never appears', async () => {
  await assert.rejects(pollAfterNavigation(async () => { throw new Error('unexpected'); }), /unexpected/);
  await assert.rejects(pollAfterNavigation(async () => false, { interval: 1, timeout: 10 }), /Timed out/);
});

test('a navigation interruption on mouse release still requires the rendered-frame poll', async () => {
  const events = [];
  await clickRetry(async (_method, { type }) => {
    events.push(type);
    if (type === 'mouseReleased') throw Object.assign(new Error('Inspected target navigated or closed'), { code: -32000 });
  }, { x: 1, y: 2 });
  assert.deepEqual(events, ['mousePressed', 'mouseReleased']);
  await assert.rejects(pollAfterNavigation(async () => false, { interval: 1, timeout: 10 }), /Timed out/);
  await assert.rejects(clickRetry(async () => { throw new Error('input failed'); }, { x: 1, y: 2 }), /input failed/);
});
