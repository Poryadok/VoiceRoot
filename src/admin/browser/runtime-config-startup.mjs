// Startup diagnostics and owned-process cleanup for the runtime config browser.
import { spawn } from 'node:child_process';
import { readFile, readdir, stat } from 'node:fs/promises';
import { join } from 'node:path';

const delay = ms => new Promise(resolve => setTimeout(resolve, ms));

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

export function startChrome(binary, args, profile) {
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
        const profileProcessesAlive = async () => {
          // This identifies observed Chrome/crashpad profile bindings; it is not
          // OS-enforced containment of arbitrary descendants that erase all markers.
          if (process.platform !== 'linux' || typeof process.getuid !== 'function') return false;
          for (const pid of await readdir('/proc')) {
            if (!/^\d+$/.test(pid) || Number(pid) === process.pid) continue;
            try {
              if ((await stat('/proc/' + pid)).uid !== process.getuid()) continue;
              const env = await readFile('/proc/' + pid + '/environ');
              if (env.toString().split('\0').includes('XDG_CONFIG_HOME=' + marker)) return true;
              const args = (await readFile('/proc/' + pid + '/cmdline')).toString().split('\0');
              if (args.some(arg => arg === profile || arg === '--user-data-dir=' + profile ||
                  arg.startsWith('--database=' + profile + '/'))) return true;
            } catch (error) {
              if (error.code !== 'ENOENT' && error.code !== 'ESRCH') throw error;
            }
          }
          return false;
        };
        const wait = async ms => {
          const deadline = Date.now() + ms;
          while (Date.now() < deadline) {
            if (didClose && !groupAlive() && !await profileProcessesAlive()) return true;
            await delay(50);
          }
          return didClose && !groupAlive() && !await profileProcessesAlive();
        };
        signal('SIGTERM');
        if (!await wait(2000)) {
          signal('SIGKILL');
          if (!await wait(5000)) throw new Error('Chrome shutdown not confirmed; profile retained');
        }
        // Outside Linux the parent is settled, but descendant observation is unavailable.
        return process.platform === 'linux' && typeof process.getuid === 'function';
      })();
      return closePromise;
    },
  };
}
