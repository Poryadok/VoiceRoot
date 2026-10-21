import assert from 'node:assert/strict';
import test from 'node:test';
import { runInNewContext } from 'node:vm';
import { bodyTextIncludesExpression } from './text-readiness.mjs';

const expected = 'Sign in with your Voice staff account.';

test('navigation without a document body is not ready, not a script failure', () => {
  assert.equal(runInNewContext(bodyTextIncludesExpression(expected), { document: { body: null } }), false);
});

test('polling becomes ready only when the new body contains the expected text', () => {
  const document = { body: null };
  const expression = bodyTextIncludesExpression(expected);
  const observations = [runInNewContext(expression, { document })];
  for (const innerText of ['', 'Signing in…\ninvalid_state', expected]) {
    document.body = { innerText };
    observations.push(runInNewContext(expression, { document }));
  }
  assert.deepEqual(observations, [false, false, false, true]);
});

test('unexpected DOM failures still throw instead of becoming not-ready', () => {
  const body = { get innerText() { throw new Error('unexpected DOM failure'); } };
  assert.throws(() => runInNewContext(bodyTextIncludesExpression(expected), { document: { body } }),
    /unexpected DOM failure/);
});
