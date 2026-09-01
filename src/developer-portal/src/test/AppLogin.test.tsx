import { beforeEach, describe, expect, it, vi, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { App } from '../App';

function mockFetchEmptyBots() {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ bot_list: { bots: [] } }), { status: 200 }),
    ),
  );
}

describe('App login screen', () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  beforeEach(() => {
    sessionStorage.clear();
    mockFetchEmptyBots();
    vi.stubEnv('VITE_OAUTH_DISABLED', 'true');
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: {
        ...window.location,
        pathname: '/',
      },
    });
  });

  it('shows JWT paste form when oauth is disabled', () => {
    render(<App />);

    expect(screen.getByRole('heading', { name: 'Voice Developer Portal' })).toBeInTheDocument();
    expect(screen.getByText('User JWT (dev only)')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Use JWT' })).toBeInTheDocument();
  });

  it('stores pasted JWT and marks user logged in', async () => {
    render(<App />);

    fireEvent.change(screen.getByPlaceholderText('Bearer access token'), {
      target: { value: ' pasted-user-jwt ' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Use JWT' }));

    expect(sessionStorage.getItem('voice_access_token')).toBe('pasted-user-jwt');
    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Your bots' })).toBeInTheDocument();
    });
  });

  it('shows status when use JWT clicked with empty token', () => {
    render(<App />);

    fireEvent.click(screen.getByRole('button', { name: 'Use JWT' }));

    expect(screen.getByText('Paste a JWT first')).toBeInTheDocument();
  });

  it('shows OAuth sign-in button when oauth enabled', async () => {
    cleanup();
    vi.unstubAllGlobals();
    mockFetchEmptyBots();
    vi.stubEnv('VITE_OAUTH_DISABLED', 'false');
    vi.resetModules();
    const { App: AppWithOAuth } = await import('../App');

    render(<AppWithOAuth />);

    expect(screen.getByRole('button', { name: 'Sign in with Voice' })).toBeInTheDocument();
    expect(screen.queryByPlaceholderText('Bearer access token')).not.toBeInTheDocument();
  });
});
