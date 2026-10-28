import assert from 'node:assert/strict';
import { mkdtemp, readFile, rm, stat } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';
import { observeProfileProcesses, startChrome } from './runtime-config-startup.mjs';

// Descendant custody uses Linux /proc; non-Linux retains the profile instead.
const linuxTest = (name, fn) => test(name, { skip: process.platform !== 'linux' }, fn);

linuxTest('unreadable proc remains unknown and cannot mask a known owned process', async () => {
  const inaccessible = Object.assign(new Error('denied'), { code: 'EACCES' });
  const fs = {
    readdir: async () => ['101', '102'],
    stat: async () => ({ uid: 42 }),
    readFile: async path => {
      if (path.includes('/101/')) throw inaccessible;
      return Buffer.from(path.endsWith('/environ') ? 'XDG_CONFIG_HOME=/profile/.chrome-config\0' : '');
    },
    uid: 42, self: -1,
  };
  assert.equal(await observeProfileProcesses('/profile', fs), 'alive');
  fs.readdir = async () => ['101'];
  assert.equal(await observeProfileProcesses('/profile', fs), 'unknown');
  fs.readFile = async path => {
    if (path.endsWith('/environ')) throw inaccessible;
    return Buffer.from('--user-data-dir=/profile\0');
  };
  assert.equal(await observeProfileProcesses('/profile', fs), 'alive');
});

linuxTest('unknown proc state settles child but keeps the profile', async () => {
  const profile = await mkdtemp(join(tmpdir(), 'voice-chrome-test-'));
  const child = startChrome(process.execPath, ['-e', 'setInterval(() => {}, 1000)'], profile, async () => 'unknown');
  try {
    assert.equal(await child.close(), false);
    assert.notEqual(child.process.exitCode ?? child.process.signalCode, null);
    assert.ok((await stat(profile)).isDirectory());
  } finally {
    await rm(profile, { recursive: true, force: true });
  }
});

linuxTest('startup timeout retains bounded redacted diagnostics and settles the child', async () => {
  const profile = await mkdtemp(join(tmpdir(), 'voice-chrome-test-'));
  const child = startChrome(process.execPath, ['-e', `
    process.on('SIGTERM', () => {});
    console.error('Authorization: Bearer SECRET_SENTINEL profile=' + process.argv[1]);
    console.error('DBus /dbus/bus.cc DevTools listening on ws://user:PASS_SENTINEL@host/?code=CODE_SENTINEL&unknown=OTHER_SENTINEL');
    console.error('x'.repeat(9000));
    require('node:fs').writeFileSync(process.argv[1] + '/started', 'ready');
    setInterval(() => {}, 1000);
  `, profile], profile);
  try {
    let started = false;
    for (let i = 0; i < 200 && !started; i++) {
      try { started = (await readFile(join(profile, 'started'), 'utf8')) === 'ready'; }
      catch { await new Promise(resolve => setTimeout(resolve, 25)); }
    }
    assert.ok(started, 'synthetic Chrome did not start');
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
  let cleanupError;
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
      try { process.kill(escapedPid, 'SIGKILL'); } catch (error) { if (error.code !== 'ESRCH') cleanupError = error; }
    }
    await rm(profile, { recursive: true, force: true });
  }
  if (cleanupError) throw cleanupError;
});
