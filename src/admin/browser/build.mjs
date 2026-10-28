// Build against the already reserved loopback fixture origin, never a live API.
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
const apiBase = new URL(process.argv[2]);
assert.equal(apiBase.protocol, 'http:');
assert.equal(apiBase.hostname, '127.0.0.1');
for (const [file, args] of [
  ['node_modules/typescript/bin/tsc', ['-b']],
  ['node_modules/vite/bin/vite.js', ['build']],
]) {
  const result = spawnSync(process.execPath, [file, ...args], {
    stdio: 'inherit',
    env: { ...process.env, VITE_RUNTIME_CONFIG_REQUIRED: 'false', VITE_VOICE_API_BASE: apiBase.origin, VITE_OAUTH_DISABLED: 'false',
      VITE_STAFF_TOKEN: '', VITE_OAUTH_CLIENT_ID: 'voice-admin' },
  });
  if (result.error) throw result.error;
  if (result.status !== 0) process.exit(result.status ?? 1);
}
