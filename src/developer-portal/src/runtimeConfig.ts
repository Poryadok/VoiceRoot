type PublicRuntimeConfig = {
  version: 1;
  app_origin: string;
  api_base_url: string;
};

let apiBase: string | undefined;
let attempted = false;

function origin(value: unknown, allowLoopback = false): string {
  if (typeof value !== 'string') throw new Error('Invalid runtime origin');
  const parsed = new URL(value);
  const loopback = parsed.hostname === '127.0.0.1' || parsed.hostname === 'localhost';
  if ((parsed.protocol !== 'https:' && !(allowLoopback && loopback && parsed.protocol === 'http:')) ||
      parsed.origin !== value || parsed.pathname !== '/' || parsed.search || parsed.hash ||
      parsed.username || parsed.password) {
    throw new Error('Invalid runtime origin');
  }
  return value;
}

export function configuredApiBase(): string {
  if (apiBase) return apiBase;
  if (!attempted && (import.meta.env.DEV || import.meta.env.VITE_RUNTIME_CONFIG_REQUIRED === 'false')) {
    const localBase = import.meta.env.VITE_VOICE_API_BASE || 'http://127.0.0.1:18080';
    return origin(localBase.replace(/\/$/, ''), true);
  }
  throw new Error('Runtime API configuration unavailable');
}

export async function loadRuntimeConfig(): Promise<void> {
  attempted = true;
  apiBase = undefined;
  const pageOrigin = origin(window.location.origin, true);
  const response = await fetch(`${pageOrigin}/voice-runtime-config.json`, {
    cache: 'no-store', credentials: 'omit', redirect: 'manual',
  });
  if (!response.ok || response.redirected || response.type === 'opaqueredirect' ||
      (response.url && response.url !== `${pageOrigin}/voice-runtime-config.json`)) {
    throw new Error('Runtime configuration unavailable');
  }
  const value: unknown = await response.json();
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('Invalid runtime configuration');
  }
  const config = value as Partial<PublicRuntimeConfig>;
  if (Object.keys(value).sort().join(',') !== 'api_base_url,app_origin,version' ||
      config.version !== 1 || origin(config.app_origin, true) !== pageOrigin) {
    throw new Error('Invalid runtime configuration');
  }
  apiBase = origin(config.api_base_url, pageOrigin.startsWith('http://'));
}

export function runtimeConfigRequired(): boolean {
  return !import.meta.env.DEV && import.meta.env.VITE_RUNTIME_CONFIG_REQUIRED !== 'false';
}
