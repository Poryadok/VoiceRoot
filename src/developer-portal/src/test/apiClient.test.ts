import { beforeEach, describe, expect, it, vi } from 'vitest';
import { setAccessToken } from '../oauth/session';

describe('api client', () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.stubEnv('VITE_VOICE_API_BASE', 'http://api.test');
    vi.resetModules();
    vi.stubGlobal('fetch', vi.fn());
  });

  it('apiFetch attaches session access token', async () => {
    const { apiFetch } = await import('../oauth/api');
    setAccessToken('session-token', 3600);
    vi.mocked(fetch).mockResolvedValue(new Response('{}', { status: 200 }));

    await apiFetch('/api/v1/bots');

    expect(fetch).toHaveBeenCalledWith(
      'http://api.test/api/v1/bots',
      expect.objectContaining({
        headers: expect.any(Headers),
      }),
    );
    const init = vi.mocked(fetch).mock.calls[0][1] as RequestInit;
    expect((init.headers as Headers).get('Authorization')).toBe(
      'Bearer session-token',
    );
  });

  it('apiFetch clears session on 401', async () => {
    const { apiFetch } = await import('../oauth/api');
    setAccessToken('expired-token', 3600);
    vi.mocked(fetch).mockResolvedValue(new Response('', { status: 401 }));

    await apiFetch('/api/v1/bots');

    expect(sessionStorage.getItem('voice_access_token')).toBeNull();
  });

  it('apiFetch sets JSON content type for body requests', async () => {
    const { apiFetch } = await import('../oauth/api');
    vi.mocked(fetch).mockResolvedValue(new Response('{}', { status: 200 }));

    await apiFetch('/api/v1/bots/manifest/validate', {
      method: 'POST',
      body: JSON.stringify({ manifest_yaml: 'name: Bot' }),
    });

    const init = vi.mocked(fetch).mock.calls[0][1] as RequestInit;
    expect((init.headers as Headers).get('Content-Type')).toBe(
      'application/json',
    );
  });

  it('apiFetch normalizes api base trailing slash', async () => {
    vi.stubEnv('VITE_VOICE_API_BASE', 'http://api.test/');
    vi.resetModules();
    const { apiFetch } = await import('../oauth/api');
    vi.mocked(fetch).mockResolvedValue(new Response('{}', { status: 200 }));

    await apiFetch('/api/v1/bots/abc/manifest');

    expect(fetch).toHaveBeenCalledWith(
      'http://api.test/api/v1/bots/abc/manifest',
      expect.any(Object),
    );
  });
});
