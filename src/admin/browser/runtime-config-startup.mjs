// Startup diagnostics and owned-process cleanup for the runtime config browser.
import { spawn } from 'node:child_process';
import { open, readFile, readdir, stat } from 'node:fs/promises';
import { join } from 'node:path';

const delay = ms => new Promise(resolve => setTimeout(resolve, ms));

async function readProc(path) {
  const handle = await open(path, 'r');
  try {
    const buffer = Buffer.alloc(65537);
    let used = 0;
    while (used < buffer.length) {
      const { bytesRead } = await handle.read(buffer, used, buffer.length - used, null);
      if (!bytesRead) return buffer.subarray(0, used);
      used += bytesRead;
    }
    throw new Error('proc observation exceeds bound');
  } finally {
    await handle.close();
  }
}

export async function observeProfileProcesses(profile, fs = {
  readdir, stat, readFile: readProc, uid: process.getuid?.(), self: process.pid,
}) {
  const marker = join(profile, '.chrome-config');
  let uncertain = false;
  let pids;
  try { pids = await fs.readdir('/proc'); }
  catch { return 'unknown'; }
  for (const pid of pids) {
    if (!/^\d+$/.test(pid) || Number(pid) === fs.self) continue;
    try {
      if ((await fs.stat('/proc/' + pid)).uid !== fs.uid) continue;
    } catch (error) {
      if (error.code !== 'ENOENT' && error.code !== 'ESRCH') uncertain = true;
      continue;
    }
    for (const file of ['environ', 'cmdline']) {
      try {
        const values = (await fs.readFile('/proc/' + pid + '/' + file)).toString().split('\0');
        if (file === 'environ' && values.includes('XDG_CONFIG_HOME=' + marker)) return 'alive';
        if (file === 'cmdline' && values.some(arg => arg === profile || arg === '--user-data-dir=' + profile ||
            arg.startsWith('--database=' + profile + '/'))) return 'alive';
      } catch (error) {
        if (error.code !== 'ENOENT' && error.code !== 'ESRCH') uncertain = true;
      }
    }
  }
  return uncertain ? 'unknown' : 'clear';
}

function safeStderr(raw) {
  // Report only fixed signatures. Chrome stderr may contain arbitrary URLs,
  // credentials or local paths, none of which should be emitted verbatim.
  return [
    ['DevTools listening', /DevTools listening on /],
    ['DBus error', /(?:dbus\/bus\.cc|dbus\/object_proxy\.cc)/i],
    ['sandbox error', /(?:no usable sandbox|sandbox.*(?:failed|error))/i],
    ['resource exhaustion', /(?:out of memory|resource temporarily unavailable)/i],
  ].filter(([, pattern]) => pattern.test(raw)).map(([name]) => name).join('; ');
}

export function startChrome(binary, args, profile, observe = observeProfileProcesses) {
  const start = Date.now();
  const marker = join(profile, '.chrome-config');
  const child = spawn(binary, args, { stdio: ['ignore', 'ignore', 'pipe'],
                                    detached: process.platform !== 'win32',
                                    env: { ...process.env, XDG_CONFIG_HOME: marker } });
  let stderr = '', stderrTruncated = false, spawnError, state = 'starting', elapsedMs = 0;
  child.stderr.setEncoding('utf8');
  child.stderr.on('data', chunk => {
    // Keep only a bounded prefix; redact once so split credentials are handled.
    stderrTruncated ||= Buffer.byteLength(stderr) + Buffer.byteLength(chunk) > 8192;
    if (Buffer.byteLength(stderr) < 8192) stderr = Buffer.from(stderr + chunk).subarray(0, 8192).toString('utf8');
  });
  child.on('error', error => { spawnError = error.code; });
  let didClose = false;
  const closed = new Promise(resolve => child.once('close', () => { didClose = true; resolve(); }));
  let closePromise;
  return {
    process: child,
    async port(timeoutMs = 15000) {
      const deadline = Date.now() + timeoutMs;
      while (Date.now() < deadline) {
        if (spawnError) { state = 'spawn_failed'; break; }
        if (child.exitCode !== null || child.signalCode !== null) { state = 'early_exit'; break; }
        try {
          const port = Number((await readFile(join(profile, 'DevToolsActivePort'), 'utf8')).split('\n')[0]);
          state = port ? 'ready' : 'port_invalid';
          elapsedMs = Date.now() - start;
          return port || undefined;
        } catch {
          await delay(Math.min(100, Math.max(0, deadline - Date.now())));
        }
      }
      if (state === 'starting') state = spawnError ? 'spawn_failed' : 'port_missing';
      elapsedMs = Date.now() - start;
      return undefined;
    },
    diagnostic() {
      return { state, elapsed_ms: elapsedMs, exit_code: child.exitCode, signal: child.signalCode,
               spawn_error: ['ENOENT', 'EACCES', 'ENOEXEC'].includes(spawnError) ? spawnError : spawnError ? 'other' : null,
               stderr: safeStderr(stderr), stderr_truncated: stderrTruncated };
    },
    close() {
      if (closePromise) return closePromise;
      closePromise = (async () => {
        if (!child.pid) { await closed; return true; }
        const signal = name => {
          try {
            if (process.platform !== 'win32') process.kill(-child.pid, name);
            else child.kill(name);
          } catch (error) {
            if (error.code !== 'ESRCH') throw error;
          }
        };
        const groupAlive = () => {
          if (process.platform === 'win32') return !didClose;
          try { process.kill(-child.pid, 0); return true; }
          catch (error) { if (error.code === 'ESRCH') return false; throw error; }
        };
        // These profile bindings do not contain arbitrary descendants that erase all markers.
        const wait = async ms => {
          const deadline = Date.now() + ms;
          while (Date.now() < deadline) {
            if (didClose && !groupAlive()) {
              if (process.platform !== 'linux' || typeof process.getuid !== 'function') return 'unknown';
              const observed = await observe(profile);
              if (observed === 'clear' || observed === 'unknown') return observed;
              if (observed !== 'alive') return 'unknown';
            }
            await delay(50);
          }
          return 'waiting';
        };
        signal('SIGTERM');
        let result = await wait(2000);
        if (result === 'waiting') {
          signal('SIGKILL');
          result = await wait(5000);
          if (result === 'waiting') throw new Error('Chrome shutdown not confirmed; profile retained');
        }
        return result === 'clear';
      })();
      return closePromise;
    },
  };
}
