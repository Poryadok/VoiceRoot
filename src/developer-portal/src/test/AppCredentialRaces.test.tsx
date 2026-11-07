import { afterEach, expect, it, vi } from 'vitest';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from '../App';
import { setAccessToken } from '../oauth/session';

const A = '00000000-0000-0000-0000-000000000001';
const B = '00000000-0000-0000-0000-000000000002';
const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status });
function deferred() {
  let resolve!: (response: Response) => void;
  const promise = new Promise<Response>((done) => { resolve = done; });
  return { promise, resolve };
}
afterEach(() => {
  cleanup();
  sessionStorage.clear();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});
function setup(fetchMock: ReturnType<typeof vi.fn>) {
  vi.stubEnv('VITE_OAUTH_DISABLED', 'true');
  setAccessToken('owner-a');
  vi.stubGlobal('fetch', fetchMock);
  render(<App />);
}

it.each(['token', 'webhook-secret'])('keeps A selected and shows its delayed %s rotation', async (credential) => {
  const pending = deferred();
  const fetchMock = vi.fn((url: string) => {
    const path = new URL(url).pathname;
    if (path.endsWith('/regenerate')) return pending.promise;
    if (path === '/api/v1/bots') return Promise.resolve(json({ bot_list: { bots: [{ id: A, name: 'Bot A' }, { id: B, name: 'Bot B' }] } }));
    if (path.endsWith('/commands')) return Promise.resolve(json({ command_list: { commands_json: '[]' } }));
    if (path.endsWith('/manifest')) return Promise.resolve(json({ manifest_yaml: '' }));
    return Promise.resolve(json({ bot: { id: path.split('/').pop(), name: 'Bot A', scopes_json: '[]' } }));
  });
  setup(fetchMock);
  const buttonName = credential === 'token' ? 'Revoke & regenerate bot token' : 'Rotate webhook secret';
  await screen.findByDisplayValue('Bot A');
  fireEvent.click(screen.getByRole('button', { name: buttonName }));
  const selectB = screen.getByRole('button', { name: 'Bot B' });
  expect(selectB).toBeDisabled();
  fireEvent.click(selectB);
  await act(async () => { pending.resolve(json({ token_response: { token: 'replacement-token' }, webhook_secret_response: { webhook_secret: 'replacement-webhook' } })); });
  expect(await screen.findByRole('dialog')).toHaveTextContent(credential === 'token' ? 'replacement-token' : 'replacement-webhook');
  expect(fetchMock.mock.calls.filter(([url]) => new URL(url).pathname === `/api/v1/bots/${B}`)).toHaveLength(0);
});

it.each(['token', 'webhook-secret'])('single-flights %s rotation so a reversed second response cannot revoke the displayed secret', async (credential) => {
  const first = deferred();
  const second = deferred();
  let rotations = 0;
  const fetchMock = vi.fn((url: string) => {
    const path = new URL(url).pathname;
    if (path.endsWith('/regenerate')) return ++rotations === 1 ? first.promise : second.promise;
    if (path === '/api/v1/bots') return Promise.resolve(json({ bot_list: { bots: [{ id: A, name: 'Bot A' }] } }));
    if (path.endsWith('/commands')) return Promise.resolve(json({ command_list: { commands_json: '[]' } }));
    if (path.endsWith('/manifest')) return Promise.resolve(json({ manifest_yaml: '' }));
    return Promise.resolve(json({ bot: { id: A, name: 'Bot A', scopes_json: '[]' } }));
  });
  setup(fetchMock);
  await screen.findByDisplayValue('Bot A');
  const button = screen.getByRole('button', { name: credential === 'token' ? 'Revoke & regenerate bot token' : 'Rotate webhook secret' });
  // Both activations occur in the same React batch, before disabled state can render.
  act(() => { fireEvent.click(button); fireEvent.click(button); });
  expect(rotations).toBe(1);
  await act(async () => {
    second.resolve(json({ token_response: { token: 'later-token' }, webhook_secret_response: { webhook_secret: 'later-webhook' } }));
    first.resolve(json({ token_response: { token: 'only-token' }, webhook_secret_response: { webhook_secret: 'only-webhook' } }));
  });
  expect(await screen.findByRole('dialog')).toHaveTextContent(credential === 'token' ? 'only-token' : 'only-webhook');
});

it('preserves a new login after the previous owner list request returns 401', async () => {
  const old = deferred();
  const fetchMock = vi.fn().mockReturnValueOnce(old.promise).mockResolvedValue(json({ bot_list: { bots: [] } }));
  setup(fetchMock);
  fireEvent.click(screen.getByRole('button', { name: 'Sign out' }));
  fireEvent.change(screen.getByPlaceholderText('Bearer access token'), { target: { value: 'owner-b' } });
  fireEvent.click(screen.getByRole('button', { name: 'Use JWT' }));
  await act(async () => { old.resolve(json({}, 401)); });
  expect(sessionStorage.getItem('voice_access_token')).toBe('owner-b');
  expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument();
});

it('shows sign-in when the current owner request returns 401', async () => {
  setup(vi.fn().mockResolvedValue(json({}, 401)));
  await waitFor(() => expect(screen.getByRole('button', { name: 'Use JWT' })).toBeInTheDocument());
});

it('preserves the rotated secret when a previous save refreshes the selected bot', async () => {
  const saved = deferred();
  const rotation = deferred();
  let lists = 0;
  const fetchMock = vi.fn((url: string, init?: RequestInit) => {
    const path = new URL(url).pathname;
    if (init?.method === 'PATCH') return saved.promise;
    if (path.endsWith('/regenerate')) return rotation.promise;
    if (path === '/api/v1/bots') {
      lists += 1;
      return Promise.resolve(json({ bot_list: { bots: [{ id: A, name: 'Bot A' }] } }));
    }
    if (path.endsWith('/commands')) return Promise.resolve(json({ command_list: { commands_json: '[]' } }));
    if (path.endsWith('/manifest')) return Promise.resolve(json({ manifest_yaml: '' }));
    return Promise.resolve(json({ bot: { id: A, name: 'Bot A', scopes_json: '[]' } }));
  });
  setup(fetchMock);
  await screen.findByDisplayValue('Bot A');
  fireEvent.click(screen.getByRole('button', { name: 'Save bot changes' }));
  fireEvent.click(screen.getByRole('button', { name: 'Revoke & regenerate bot token' }));
  await act(async () => { saved.resolve(json({ bot: { id: A, name: 'Bot A' } })); });
  await waitFor(() => expect(lists).toBe(2));
  await act(async () => { rotation.resolve(json({ token_response: { token: 'after-save-token' } })); });
  expect(await screen.findByRole('dialog')).toHaveTextContent('after-save-token');
});

it.each(['registration', 'rotation'])('serializes registration and rotation when %s starts first', async (first) => {
  const pending = deferred();
  let mutations = 0;
  const fetchMock = vi.fn((url: string, init?: RequestInit) => {
    const path = new URL(url).pathname;
    if (init?.method === 'POST') { mutations += 1; return pending.promise; }
    if (path === '/api/v1/bots') return Promise.resolve(json({ bot_list: { bots: [{ id: A, name: 'Bot A' }] } }));
    if (path.endsWith('/commands')) return Promise.resolve(json({ command_list: { commands_json: '[]' } }));
    if (path.endsWith('/manifest')) return Promise.resolve(json({ manifest_yaml: '' }));
    return Promise.resolve(json({ bot: { id: A, name: 'Bot A', scopes_json: '[]' } }));
  });
  setup(fetchMock);
  await screen.findByDisplayValue('Bot A');
  fireEvent.change(screen.getByPlaceholderText('MyBot'), { target: { value: 'Bot B' } });
  const register = screen.getByRole('button', { name: 'Register bot' });
  const rotate = screen.getByRole('button', { name: 'Revoke & regenerate bot token' });
  act(() => {
    fireEvent.click(first === 'registration' ? register : rotate);
    fireEvent.click(first === 'registration' ? rotate : register);
  });
  expect(mutations).toBe(1);
  await act(async () => { pending.resolve(json({ bot: { id: B, name: 'Bot B' }, token_response: { token: 'only-mutation-token' } })); });
  expect(await screen.findByRole('dialog')).toHaveTextContent('only-mutation-token');
});

it('shows sign-in when the current owner session expires without another API request', async () => {
  vi.useFakeTimers();
  vi.stubEnv('VITE_OAUTH_DISABLED', 'true');
  setAccessToken('owner-a', 1);
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(json({ bot_list: { bots: [] } })));
  render(<App />);
  await act(async () => { await vi.advanceTimersByTimeAsync(1001); });
  expect(screen.getByRole('button', { name: 'Use JWT' })).toBeInTheDocument();
});
