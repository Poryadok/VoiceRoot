import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiBaseUrl } from '../oauth/api';
import { loadRuntimeConfig } from '../runtimeConfig';

describe('release runtime configuration', () => {
  beforeEach(() => vi.unstubAllGlobals());

  it('binds an explicit API origin to this page before API use', async () => {
    const origin = window.location.origin;
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      version: 1, app_origin: origin, api_base_url: 'https://api-dev.comrade.click',
    }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    await loadRuntimeConfig();
    expect(fetchMock).toHaveBeenCalledWith(`${origin}/voice-runtime-config.json`, {
      cache: 'no-store', credentials: 'omit', redirect: 'manual',
    });
    expect(apiBaseUrl()).toBe('https://api-dev.comrade.click');
  });

  it.each([
    ['missing', 404, null],
    ['wrong origin', 200, { version: 1, app_origin: 'https://developers.comrade.click', api_base_url: 'https://voice.comrade.click' }],
    ['invalid API', 200, { version: 1, app_origin: window.location.origin, api_base_url: 'https://voice.comrade.click/path' }],
  ])('rejects %s without an API endpoint', async (_name, status, body) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(body && JSON.stringify(body), { status })));
    await expect(loadRuntimeConfig()).rejects.toThrow();
    expect(() => apiBaseUrl()).toThrow();
  });
});
