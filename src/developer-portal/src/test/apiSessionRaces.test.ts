import { afterEach, expect, it, vi } from 'vitest';
import { apiFetch } from '../oauth/api';
import { setAccessToken } from '../oauth/session';

afterEach(() => { sessionStorage.clear(); vi.unstubAllGlobals(); });
it.each(['replacement-token', 'original-token'])('keeps replacement session %s after an old request returns 401', async (replacement) => {
  let resolve!: (response: Response) => void;
  vi.stubGlobal('fetch', vi.fn(() => new Promise<Response>((done) => { resolve = done; })));
  setAccessToken('original-token');
  const request = apiFetch('/api/v1/test');
  setAccessToken(replacement);
  resolve(new Response('', { status: 401 }));
  await request;
  expect(sessionStorage.getItem('voice_access_token')).toBe(replacement);
});
