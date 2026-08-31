import { beforeEach, describe, expect, it, vi, afterEach } from 'vitest';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { App } from '../App';

describe('App login screen', () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    sessionStorage.clear();
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

  it('stores pasted JWT and marks user logged in', () => {
    render(<App />);

    fireEvent.change(screen.getByPlaceholderText('Bearer access token'), {
      target: { value: ' pasted-user-jwt ' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Use JWT' }));

    expect(sessionStorage.getItem('voice_access_token')).toBe('pasted-user-jwt');
    expect(screen.getByRole('heading', { name: 'Your bots' })).toBeInTheDocument();
  });

  it('shows status when use JWT clicked with empty token', () => {
    render(<App />);

    fireEvent.click(screen.getByRole('button', { name: 'Use JWT' }));

    expect(screen.getByText('Paste a JWT first')).toBeInTheDocument();
  });

  it('shows OAuth sign-in button when oauth enabled', async () => {
    cleanup();
    vi.stubEnv('VITE_OAUTH_DISABLED', 'false');
    vi.resetModules();
    const { App: AppWithOAuth } = await import('../App');

    render(<AppWithOAuth />);

    expect(screen.getByRole('button', { name: 'Sign in with Voice' })).toBeInTheDocument();
    expect(screen.queryByPlaceholderText('Bearer access token')).not.toBeInTheDocument();
  });
});
