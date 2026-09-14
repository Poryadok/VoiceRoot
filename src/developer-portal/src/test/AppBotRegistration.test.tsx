import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { App } from '../App';

const BOT_A = '00000000-0000-0000-0000-000000000001';
const BOT_B = '00000000-0000-0000-0000-000000000002';

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status });
}

function hasPath(url: unknown, pathname: string) {
  return typeof url === 'string' && new URL(url).pathname === pathname;
}

function setupLoggedIn(fetchMock: ReturnType<typeof vi.fn>) {
  sessionStorage.setItem('voice_access_token', 'test-jwt');
  vi.stubEnv('VITE_OAUTH_DISABLED', 'true');
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...window.location, pathname: '/' },
  });
  vi.stubGlobal('fetch', fetchMock);
}

function botDetailResponse(id: string, name: string, scopesJson: string) {
  return jsonResponse({
    bot: { id, name, description: `${name} desc`, scopes_json: scopesJson },
  });
}

describe('App bot registration and selection', () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    sessionStorage.clear();
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    sessionStorage.clear();
  });

  it('registers bot with form values', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ bot_list: { bots: [] } }))
      .mockResolvedValueOnce(
        jsonResponse({
          bot: { id: BOT_A, name: 'Custom Bot' },
          token_response: { token: 'new-token' },
          webhook_secret_response: { webhook_secret: 'wh-sec' },
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ bot_list: { bots: [{ id: BOT_A, name: 'Custom Bot' }] } }))
      .mockResolvedValueOnce(botDetailResponse(BOT_A, 'Custom Bot', '["DM_SEND"]'))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }));

    setupLoggedIn(fetchMock);
    render(<App />);

    await waitFor(() => {
      expect(screen.getByTestId('bot-register')).toBeInTheDocument();
    });

    const regSection = screen.getByTestId('bot-register');
    fireEvent.change(regSection.querySelector('input[placeholder="MyBot"]')!, {
      target: { value: 'Custom Bot' },
    });
    fireEvent.change(regSection.querySelector('input[placeholder="What this bot does"]')!, {
      target: { value: 'Does things' },
    });
    fireEvent.change(regSection.querySelector('input[placeholder=\'["TEXT_CHAT_SEND_MESSAGES"]\']')!, {
      target: { value: '["DM_SEND"]' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Register bot' }));

    await waitFor(() => {
      expect(screen.getByText(/Registered bot/)).toBeInTheDocument();
    });

    const postCall = fetchMock.mock.calls.find(
      ([url, init]) =>
        hasPath(url, '/api/v1/bots') &&
        (init as RequestInit)?.method === 'POST',
    );
    expect(postCall).toBeTruthy();
    expect(JSON.parse((postCall![1] as RequestInit).body as string)).toEqual({
      name: 'Custom Bot',
      description: 'Does things',
      scopes_json: '["DM_SEND"]',
    });
  });

  it('GETs bot detail on selection and populates scopes', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          bot_list: {
            bots: [
              { id: BOT_A, name: 'Bot A' },
              { id: BOT_B, name: 'Bot B' },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(
        botDetailResponse(BOT_A, 'Bot A', '["TEXT_CHAT_SEND_MESSAGES"]'),
      )
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }))
      .mockResolvedValueOnce(
        botDetailResponse(BOT_B, 'Bot B', '["TEXT_CHAT_READ_HISTORY"]'),
      )
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }));

    setupLoggedIn(fetchMock);
    render(<App />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('["TEXT_CHAT_SEND_MESSAGES"]')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Bot B' }));

    await waitFor(() => {
      expect(screen.getByDisplayValue('["TEXT_CHAT_READ_HISTORY"]')).toBeInTheDocument();
    });

    const getCalls = fetchMock.mock.calls.filter(
      ([url, init]) =>
        typeof url === 'string' &&
        url.includes('/api/v1/bots/') &&
        !(init as RequestInit | undefined)?.method,
    );
    expect(getCalls.some(([url]) => url.endsWith(BOT_A))).toBe(true);
    expect(getCalls.some(([url]) => url.endsWith(BOT_B))).toBe(true);
  });

  it('clears one-shot secrets when switching bots', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          bot_list: {
            bots: [
              { id: BOT_A, name: 'Bot A' },
              { id: BOT_B, name: 'Bot B' },
            ],
          },
        }),
      )
      .mockResolvedValueOnce(
        botDetailResponse(BOT_A, 'Bot A', '["TEXT_CHAT_SEND_MESSAGES"]'),
      )
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }))
      .mockResolvedValueOnce(
        jsonResponse({ token_response: { token: 'visible-token' } }),
      )
      .mockResolvedValueOnce(
        botDetailResponse(BOT_B, 'Bot B', '["TEXT_CHAT_SEND_MESSAGES"]'),
      )
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }));

    setupLoggedIn(fetchMock);
    render(<App />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Revoke & regenerate bot token' })).toBeEnabled();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Revoke & regenerate bot token' }));

    await waitFor(() => {
      expect(screen.getByText('visible-token')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole('button', { name: 'Close and clear' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Copy one-shot secrets' })).not.toBeInTheDocument());
    fireEvent.click(screen.getByRole('button', { name: 'Bot B' }));

    await waitFor(() => {
      expect(screen.queryByText('visible-token')).not.toBeInTheDocument();
    });
  });

  it('shows registration secrets in a copy-once dialog and clears each after copying', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ bot_list: { bots: [] } }))
      .mockResolvedValueOnce(
        jsonResponse({
          bot: { id: BOT_A, name: 'Secret Bot' },
          token_response: { token: 'registration-token' },
          webhook_secret_response: { webhook_secret: 'registration-webhook-secret' },
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ bot_list: { bots: [{ id: BOT_A, name: 'Secret Bot' }] } }))
      .mockResolvedValueOnce(botDetailResponse(BOT_A, 'Secret Bot', '[]'))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }));

    setupLoggedIn(fetchMock);
    render(<App />);

    await waitFor(() => expect(screen.getByTestId('bot-register')).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText('Bot name'), { target: { value: 'Secret Bot' } });
    fireEvent.click(screen.getByRole('button', { name: 'Register bot' }));

    const dialog = await screen.findByRole('dialog', { name: 'Copy one-shot secrets' });
    expect(dialog).toHaveTextContent('registration-token');
    expect(dialog).toHaveTextContent('registration-webhook-secret');
    expect(screen.queryByText('Bot token (shown once):')).not.toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole('button', { name: 'Copy bot token' }));

    await waitFor(() => expect(writeText).toHaveBeenCalledWith('registration-token'));
    expect(dialog).not.toHaveTextContent('registration-token');
    expect(dialog).toHaveTextContent('registration-webhook-secret');

    fireEvent.click(within(dialog).getByRole('button', { name: 'Copy webhook secret' }));

    await waitFor(() => expect(writeText).toHaveBeenCalledWith('registration-webhook-secret'));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Copy one-shot secrets' })).not.toBeInTheDocument());
  });

  it('moves focus into the one-shot-secret dialog, traps Tab, and restores the opener after Escape clears secrets', async () => {
    const user = userEvent.setup();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ bot_list: { bots: [] } }))
      .mockResolvedValueOnce(
        jsonResponse({
          bot: { id: BOT_A, name: 'Focus Bot' },
          token_response: { token: 'focus-token' },
          webhook_secret_response: { webhook_secret: 'focus-webhook-secret' },
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ bot_list: { bots: [{ id: BOT_A, name: 'Focus Bot' }] } }))
      .mockResolvedValueOnce(botDetailResponse(BOT_A, 'Focus Bot', '[]'))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }));

    setupLoggedIn(fetchMock);
    render(<App />);

    await waitFor(() => expect(screen.getByTestId('bot-register')).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText('Bot name'), { target: { value: 'Focus Bot' } });
    const opener = screen.getByRole('button', { name: 'Register bot' });
    opener.focus();
    await user.click(opener);

    const dialog = await screen.findByRole('dialog', { name: 'Copy one-shot secrets' });
    const firstControl = within(dialog).getByRole('button', { name: 'Copy bot token' });
    const lastControl = within(dialog).getByRole('button', { name: 'Close and clear' });
    expect(document.activeElement).toBe(firstControl);

    await user.tab();
    expect(document.activeElement).toBe(within(dialog).getByRole('button', { name: 'Copy webhook secret' }));
    await user.tab();
    expect(document.activeElement).toBe(lastControl);
    await user.tab();
    expect(document.activeElement).toBe(firstControl);

    await user.tab({ shift: true });
    expect(document.activeElement).toBe(lastControl);

    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Copy one-shot secrets' })).not.toBeInTheDocument());
    expect(screen.queryByText('focus-token')).not.toBeInTheDocument();
    expect(screen.queryByText('focus-webhook-secret')).not.toBeInTheDocument();
    expect(document.activeElement).toBe(opener);
  });

  it('keeps focus in the dialog after copying one of two secrets, then traps Tab and Escape clears the remaining secret', async () => {
    const user = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ bot_list: { bots: [] } }))
      .mockResolvedValueOnce(
        jsonResponse({
          bot: { id: BOT_A, name: 'Partial copy Bot' },
          token_response: { token: 'partial-copy-token' },
          webhook_secret_response: { webhook_secret: 'partial-copy-webhook-secret' },
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ bot_list: { bots: [{ id: BOT_A, name: 'Partial copy Bot' }] } }))
      .mockResolvedValueOnce(botDetailResponse(BOT_A, 'Partial copy Bot', '[]'))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }));

    setupLoggedIn(fetchMock);
    render(<App />);

    await waitFor(() => expect(screen.getByTestId('bot-register')).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText('Bot name'), { target: { value: 'Partial copy Bot' } });
    await user.click(screen.getByRole('button', { name: 'Register bot' }));

    const dialog = await screen.findByRole('dialog', { name: 'Copy one-shot secrets' });
    await user.click(within(dialog).getByRole('button', { name: 'Copy bot token' }));

    await waitFor(() => expect(writeText).toHaveBeenCalledWith('partial-copy-token'));
    const remainingCopy = within(dialog).getByRole('button', { name: 'Copy webhook secret' });
    const close = within(dialog).getByRole('button', { name: 'Close and clear' });
    await waitFor(() => expect(document.activeElement).toBe(remainingCopy));

    await user.tab();
    expect(document.activeElement).toBe(close);
    await user.tab();
    expect(document.activeElement).toBe(remainingCopy);
    await user.tab({ shift: true });
    expect(document.activeElement).toBe(close);

    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Copy one-shot secrets' })).not.toBeInTheDocument());
    expect(screen.queryByText('partial-copy-webhook-secret')).not.toBeInTheDocument();
  });

  it('makes the whole portal background inaccessible while the one-shot-secret dialog is open', async () => {
    const user = userEvent.setup();
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          bot_list: { bots: [{ id: BOT_A, name: 'Background Bot' }] },
        }),
      )
      .mockResolvedValueOnce(botDetailResponse(BOT_A, 'Background Bot', '[]'))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }))
      .mockResolvedValueOnce(jsonResponse({ token_response: { token: 'background-token' } }));

    setupLoggedIn(fetchMock);
    render(<App />);

    await waitFor(() => expect(screen.getByRole('button', { name: 'Revoke & regenerate bot token' })).toBeEnabled());
    const regenerateButton = screen.getByRole('button', { name: 'Revoke & regenerate bot token' });
    await user.click(regenerateButton);
    const dialog = await screen.findByRole('dialog', { name: 'Copy one-shot secrets' });
    const portalBackground = screen.getByRole('main', { hidden: true });
    const rotateButton = within(portalBackground).getByRole('button', { name: 'Rotate webhook secret', hidden: true });
    const signOutButton = within(portalBackground).getByRole('button', { name: 'Sign out', hidden: true });

    expect(portalBackground).toHaveAttribute('inert');
    expect(portalBackground).toHaveAttribute('aria-hidden', 'true');
    expect(portalBackground).not.toContainElement(dialog);
    expect(rotateButton).toBeInTheDocument();
    expect(signOutButton).toBeInTheDocument();

    await user.tab({ shift: true });
    expect(document.activeElement).toBe(within(dialog).getByRole('button', { name: 'Close and clear' }));
    expect(document.activeElement).not.toBe(rotateButton);
    expect(document.activeElement).not.toBe(signOutButton);
    await user.click(rotateButton);
    expect(fetchMock).toHaveBeenCalledTimes(5);
    expect(screen.getByRole('dialog', { name: 'Copy one-shot secrets' })).toHaveTextContent('background-token');
  });

  it('clears both one-shot secrets and closes the dialog when logging out', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ bot_list: { bots: [] } }))
      .mockResolvedValueOnce(
        jsonResponse({
          bot: { id: BOT_A, name: 'Logout Bot' },
          token_response: { token: 'logout-token' },
          webhook_secret_response: { webhook_secret: 'logout-webhook-secret' },
        }),
      )
      .mockResolvedValueOnce(jsonResponse({ bot_list: { bots: [{ id: BOT_A, name: 'Logout Bot' }] } }))
      .mockResolvedValueOnce(botDetailResponse(BOT_A, 'Logout Bot', '[]'))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }));

    setupLoggedIn(fetchMock);
    render(<App />);

    await waitFor(() => expect(screen.getByTestId('bot-register')).toBeInTheDocument());
    fireEvent.change(screen.getByLabelText('Bot name'), { target: { value: 'Logout Bot' } });
    fireEvent.click(screen.getByRole('button', { name: 'Register bot' }));

    await screen.findByRole('dialog', { name: 'Copy one-shot secrets' });
    const portalBackground = screen.getByRole('main', { hidden: true });
    const signOutButton = screen.getByRole('button', { name: 'Sign out' });
    expect(portalBackground).not.toContainElement(signOutButton);
    fireEvent.click(signOutButton);

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Copy one-shot secrets' })).not.toBeInTheDocument());
    expect(screen.queryByText('logout-token')).not.toBeInTheDocument();
    expect(screen.queryByText('logout-webhook-secret')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Use JWT' })).toBeInTheDocument();
  });

  it('keeps a one-shot secret visible and reports a status when copying fails', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('permission denied'));
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          bot_list: { bots: [{ id: BOT_A, name: 'Clipboard Bot' }] },
        }),
      )
      .mockResolvedValueOnce(botDetailResponse(BOT_A, 'Clipboard Bot', '[]'))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }))
      .mockResolvedValueOnce(jsonResponse({ token_response: { token: 'clipboard-token' } }));

    setupLoggedIn(fetchMock);
    render(<App />);

    await waitFor(() => expect(screen.getByRole('button', { name: 'Revoke & regenerate bot token' })).toBeEnabled());
    fireEvent.click(screen.getByRole('button', { name: 'Revoke & regenerate bot token' }));
    const dialog = await screen.findByRole('dialog', { name: 'Copy one-shot secrets' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Copy bot token' }));

    await waitFor(() => expect(writeText).toHaveBeenCalledWith('clipboard-token'));
    await waitFor(() => expect(dialog).toHaveTextContent('clipboard-token'));
    expect(within(dialog).getByRole('status')).toHaveTextContent('Could not copy the secret; it remains visible until copied or dismissed.');
  });

  it('keeps a one-shot secret visible and explains when Clipboard API is unavailable', async () => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: undefined,
    });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        jsonResponse({
          bot_list: { bots: [{ id: BOT_A, name: 'No Clipboard Bot' }] },
        }),
      )
      .mockResolvedValueOnce(botDetailResponse(BOT_A, 'No Clipboard Bot', '[]'))
      .mockResolvedValueOnce(jsonResponse({ command_list: { commands_json: '[]' } }))
      .mockResolvedValueOnce(jsonResponse({ manifest_yaml: '' }))
      .mockResolvedValueOnce(jsonResponse({ token_response: { token: 'no-clipboard-token' } }));

    setupLoggedIn(fetchMock);
    render(<App />);

    await waitFor(() => expect(screen.getByRole('button', { name: 'Revoke & regenerate bot token' })).toBeEnabled());
    fireEvent.click(screen.getByRole('button', { name: 'Revoke & regenerate bot token' }));
    const dialog = await screen.findByRole('dialog', { name: 'Copy one-shot secrets' });
    fireEvent.click(within(dialog).getByRole('button', { name: 'Copy bot token' }));

    await waitFor(() => expect(dialog).toHaveTextContent('no-clipboard-token'));
    expect(screen.getByRole('status')).toHaveTextContent('Clipboard is unavailable; copy the secret manually before closing this dialog.');
  });

  it('shows privileged scope warnings in registration and edit forms', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse({ bot_list: { bots: [] } }));

    setupLoggedIn(fetchMock);
    render(<App />);

    await waitFor(() => {
      expect(screen.getByTestId('bot-register')).toBeInTheDocument();
    });

    const regSection = screen.getByTestId('bot-register');
    fireEvent.change(regSection.querySelector('input[placeholder=\'["TEXT_CHAT_SEND_MESSAGES"]\']')!, {
      target: { value: '["TEXT_CHAT_READ_HISTORY","SPACE_MANAGE_ROLES"]' },
    });

    await waitFor(() => {
      expect(screen.getByTestId('reg-scope-warnings')).toBeInTheDocument();
    });
    expect(screen.getByText(/TEXT_CHAT_READ_HISTORY is privileged/)).toBeInTheDocument();
    expect(screen.getByText(/SPACE_MANAGE_ROLES is privileged/)).toBeInTheDocument();
  });
});
