import assert from 'node:assert/strict';
import { mkdtemp, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { startChrome } from './runtime-config-startup.mjs';

// Descendant custody uses Linux /proc; non-Linux retains the profile instead.
const linuxTest = (name, fn) => test(name, { skip: process.platform !== 'linux' }, fn);

linuxTest('startup timeout retains bounded redacted diagnostics and settles the child', async () => {
  const profile = await mkdtemp(join(tmpdir(), 'voice-chrome-test-'));
  const child = startChrome(process.execPath, ['-e', `
    process.on('SIGTERM', () => {});
    console.error('Authorization: Bearer SECRET_SENTINEL profile=' + process.argv[1]);
    console.error('DBus /dbus/bus.cc DevTools listening on ws://user:PASS_SENTINEL@host/?code=CODE_SENTINEL&unknown=OTHER_SENTINEL');
    console.error('x'.repeat(9000));
    setInterval(() => {}, 1000);
  `, profile], profile);
  try {
    assert.equal(await child.port(500), undefined);
    const diagnostic = child.diagnostic();
    assert.equal(diagnostic.state, 'port_missing');
    assert.ok(diagnostic.elapsed_ms >= 0);
    assert.doesNotMatch(JSON.stringify(diagnostic), /SECRET_SENTINEL|PASS_SENTINEL|CODE_SENTINEL|OTHER_SENTINEL|voice-chrome-test-/);
    assert.equal(diagnostic.stderr, 'DevTools listening; DBus error');
    assert.equal(diagnostic.stderr_truncated, true);
    assert.ok(Buffer.byteLength(diagnostic.stderr) <= 2048);
    await child.close();
    assert.notEqual(child.process.exitCode ?? child.process.signalCode, null);
    await assert.rejects(readFile(join(profile, 'DevToolsActivePort')));
  } finally {
    await child.close();
    await rm(profile, { recursive: true, force: true });
  }
});

linuxTest('missing executable is reported without an unhandled spawn error', async () => {
  const profile = await mkdtemp(join(tmpdir(), 'voice-chrome-test-'));
  const child = startChrome('/no/such/chrome', [], profile);
  try {
    assert.equal(await child.port(500), undefined);
    assert.equal(child.diagnostic().state, 'spawn_failed');
    await child.close();
  } finally {
    await rm(profile, { recursive: true, force: true });
  }
});

for (const dropMarker of [false, true]) linuxTest(`escaped descendant prevents profile deletion (marker removed: ${dropMarker})`, async () => {
  const profile = await mkdtemp(join(tmpdir(), 'voice-chrome-test-'));
  const pidFile = join(profile, 'escaped.pid');
  const child = startChrome(process.execPath, ['-e', `
    const { spawn } = require('node:child_process');
    const { writeFileSync } = require('node:fs');
    const env = { ...process.env };
    if (${dropMarker}) delete env.XDG_CONFIG_HOME;
    const escaped = spawn(process.execPath, ['-e', 'setInterval(() => {}, 1000)', process.argv[2]], { detached: true, stdio: 'ignore', env });
    escaped.unref();
    writeFileSync(process.argv[1], String(escaped.pid));
    setInterval(() => {}, 1000);
  `, pidFile, profile], profile);
  let escapedPid;
  try {
    for (let i = 0; i < 100 && !escapedPid; i++) {
      try { escapedPid = Number(await readFile(pidFile, 'utf8')); }
      catch { await new Promise(resolve => setTimeout(resolve, 25)); }
    }
    assert.ok(escapedPid);
    await assert.rejects(child.close(), /shutdown not confirmed/);
    assert.equal(Number(await readFile(pidFile, 'utf8')), escapedPid);
  } finally {
    if (escapedPid) {
      try { process.kill(escapedPid, 'SIGKILL'); } catch (error) { if (error.code !== 'ESRCH') throw error; }
    }
    await rm(profile, { recursive: true, force: true });
  }
});
