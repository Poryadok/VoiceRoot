import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { App } from '../App';

const BOT_ID = '00000000-0000-0000-0000-000000000001';

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status });
}

function hasPath(url: unknown, pathname: string) {
  return typeof url === 'string' && new URL(url).pathname === pathname;
}

function botDetailResponse(id: string, name: string, description: string) {
  return jsonResponse({
    bot: { id, name, description, scopes_json: '[]' },
  });
}

function empty204() {
  return new Response(null, { status: 204 });
}

function setupLoggedInWithBot(fetchMock: ReturnType<typeof vi.fn>) {
  sessionStorage.setItem('voice_access_token', 'test-jwt');
  vi.stubEnv('VITE_OAUTH_DISABLED', 'true');
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, pathname: '/' },
  });
  vi.stubGlobal('fetch', fetchMock);
}

describe('App bot lifecycle UI', () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    sessionStorage.clear();
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    sessionStorage.clear();
  });

  it('PATCHes bot when update form is saved', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          bot_list: {
            bots: [{ id: BOT_ID, name: 'Old Name', description: 'Old desc' }],
          },
        }),
      )
      .mockResolvedValueOnce(botDetailResponse(BOT_ID, 'Old Name', 'Old desc'))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }))
      .mockResolvedValueOnce(
        jsonResponse({
          bot: { id: BOT_ID, name: 'New Name', description: 'New desc' },
        }),
      )
      .mockResolvedValueOnce(
        jsonResponse({
          bot_list: {
            bots: [{ id: BOT_ID, name: 'New Name', description: 'New desc' }],
          },
        }),
      )
      .mockResolvedValueOnce(botDetailResponse(BOT_ID, 'New Name', 'New desc'))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }));

    setupLoggedInWithBot(fetchMock);
    render(<App />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('Old Name')).toBeInTheDocument();
    });

    fireEvent.change(within(screen.getByTestId('bot-lifecycle')).getByLabelText('Bot name'), {
      target: { value: 'New Name' },
    });
    fireEvent.change(within(screen.getByTestId('bot-lifecycle')).getByLabelText('Bot description'), {
      target: { value: 'New desc' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Save bot changes' }));

    await waitFor(() => {
      expect(screen.getByText('Bot updated')).toBeInTheDocument();
    });

    const patchCall = fetchMock.mock.calls.find(
      ([url, init]) =>
        hasPath(url, `/api/v1/bots/${BOT_ID}`) &&
        (init as RequestInit)?.method === 'PATCH',
    );
    expect(patchCall).toBeTruthy();
    expect(JSON.parse((patchCall![1] as RequestInit).body as string)).toEqual({
      name: 'New Name',
      description: 'New desc',
      scopes_json: '[]',
    });
  });

  it('DELETEs bot after confirmation', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true);
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          bot_list: {
            bots: [{ id: BOT_ID, name: 'To Delete', description: '' }],
          },
        }),
      )
      .mockResolvedValueOnce(botDetailResponse(BOT_ID, 'To Delete', ''))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }))
      .mockResolvedValueOnce(empty204())
      .mockResolvedValueOnce(jsonResponse({ bot_list: { bots: [] } }));

    setupLoggedInWithBot(fetchMock);
    render(<App />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Delete bot' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Delete bot' }));

    await waitFor(() => {
      expect(screen.getByText('Bot deleted')).toBeInTheDocument();
    });

    expect(confirmSpy).toHaveBeenCalled();
    const deleteCall = fetchMock.mock.calls.find(
      ([url, init]) =>
        hasPath(url, `/api/v1/bots/${BOT_ID}`) &&
        (init as RequestInit)?.method === 'DELETE',
    );
    expect(deleteCall).toBeTruthy();
    expect(screen.queryByText(/Selected bot:/)).not.toBeInTheDocument();
  });

  it('does not DELETE when confirmation is cancelled', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(false);
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          bot_list: {
            bots: [{ id: BOT_ID, name: 'Keep Me', description: '' }],
          },
        }),
      )
      .mockResolvedValueOnce(botDetailResponse(BOT_ID, 'Keep Me', ''))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }));

    setupLoggedInWithBot(fetchMock);
    render(<App />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Delete bot' })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Delete bot' }));

    const deleteCall = fetchMock.mock.calls.find(
      ([, init]) => (init as RequestInit | undefined)?.method === 'DELETE',
    );
    expect(deleteCall).toBeUndefined();
  });
});
