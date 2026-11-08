import { afterEach, expect, it, vi } from 'vitest';
import { act, cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { App } from '../App';
import { apiFetch } from '../api/client';
import { setAccessToken } from '../oauth/session';

afterEach(() => {
  cleanup();
  sessionStorage.clear();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});
it.each(['replacement-token', 'original-token'])('keeps replacement session %s after an old request returns 401', async (replacement) => {
  let resolve!: (response: Response) => void;
  vi.stubGlobal('fetch', vi.fn(() => new Promise<Response>((done) => { resolve = done; })));
  setAccessToken('original-token');
  const request = apiFetch('/api/v1/test');
  setAccessToken(replacement);
  resolve(new Response('', { status: 401 }));
  await request;
  expect(sessionStorage.getItem('voice_admin_access_token')).toBe(replacement);
});
it('returns the staff shell to sign-in after its current request returns 401', async () => {
  vi.stubEnv('VITE_OAUTH_DISABLED', 'true');
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('', { status: 401 })));
  setAccessToken('staff-token');
  render(<MemoryRouter initialEntries={['/queue']}><App /></MemoryRouter>);
  expect(await screen.findByLabelText('Staff JWT')).toBeInTheDocument();
  expect(screen.queryByRole('navigation', { name: 'Main' })).not.toBeInTheDocument();
});
it('returns the staff shell to sign-in on expiry without another request', async () => {
  vi.useFakeTimers();
  vi.stubEnv('VITE_OAUTH_DISABLED', 'true');
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ report_list: { reports: [] } }))));
  setAccessToken('staff-token', 1);
  render(<MemoryRouter initialEntries={['/queue']}><App /></MemoryRouter>);
  await act(async () => { await vi.advanceTimersByTimeAsync(1001); });
  expect(screen.getByLabelText('Staff JWT')).toBeInTheDocument();
});
