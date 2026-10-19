import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, render, screen } from '@testing-library/react';
import { App } from '../App';
import * as oauthApi from '../oauth/api';
import { setOAuthState, setPkceVerifier } from '../oauth/session';

vi.mock('../oauth/api', async (importOriginal) => ({
  ...await importOriginal<typeof import('../oauth/api')>(),
  exchangeAuthorizationCode: vi.fn(),
}));

describe('Portal browser routing', () => {
  beforeEach(() => {
    sessionStorage.clear();
    vi.mocked(oauthApi.exchangeAuthorizationCode).mockReset();
    window.history.replaceState(null, '', '/');
  });

  afterEach(() => {
    cleanup();
    window.history.replaceState(null, '', '/');
  });

  it('reacts to browser history navigation without bypassing callback state checks', async () => {
    setOAuthState('expected');
    setPkceVerifier('verifier');
    render(<App />);
    expect(screen.getByRole('heading', { name: 'Voice Developer Portal' })).toBeInTheDocument();

    act(() => {
      window.history.pushState(null, '', '/callback?code=code&state=wrong');
      window.dispatchEvent(new PopStateEvent('popstate'));
    });
    expect(await screen.findByRole('alert')).toHaveTextContent('invalid_state');
    expect(oauthApi.exchangeAuthorizationCode).not.toHaveBeenCalled();
    expect(screen.queryByRole('heading', { name: 'Voice Developer Portal' })).not.toBeInTheDocument();

    act(() => {
      window.history.replaceState(null, '', '/');
      window.dispatchEvent(new PopStateEvent('popstate'));
    });
    expect(screen.getByRole('heading', { name: 'Voice Developer Portal' })).toBeInTheDocument();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('routes a direct callback with its query intact', async () => {
    window.history.replaceState(null, '', '/callback?error=access_denied');
    render(<App />);
    expect(await screen.findByRole('alert')).toHaveTextContent('access_denied');
    expect(oauthApi.exchangeAuthorizationCode).not.toHaveBeenCalled();
  });

  it.each(['/unknown', '/Callback'])('preserves the Portal fallback at %s', (path) => {
    window.history.replaceState(null, '', path);
    render(<App />);
    expect(screen.getByRole('heading', { name: 'Voice Developer Portal' })).toBeInTheDocument();
    expect(oauthApi.exchangeAuthorizationCode).not.toHaveBeenCalled();
  });
});
