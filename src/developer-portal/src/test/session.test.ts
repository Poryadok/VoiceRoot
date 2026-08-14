import { describe, expect, it, beforeEach } from 'vitest';
import {
  isLoggedIn,
  setAccessToken,
  setOAuthState,
  takeOAuthState,
  clearSession,
} from '../oauth/session';

describe('oauth session state', () => {
  beforeEach(() => {
    sessionStorage.clear();
  });

  it('stores and consumes oauth state once', () => {
    setOAuthState('state-abc');
    expect(takeOAuthState()).toBe('state-abc');
    expect(takeOAuthState()).toBeNull();
  });

  it('expires access token after expires_in', () => {
    setAccessToken('token', 1);
    expect(isLoggedIn()).toBe(true);
    const expiresAt = Number(sessionStorage.getItem('voice_token_expires_at'));
    sessionStorage.setItem('voice_token_expires_at', String(expiresAt - 2000));
    expect(isLoggedIn()).toBe(false);
    expect(sessionStorage.getItem('voice_access_token')).toBeNull();
  });

  it('clearSession removes oauth state', () => {
    setOAuthState('s');
    setAccessToken('t', 60);
    clearSession();
    expect(takeOAuthState()).toBeNull();
    expect(isLoggedIn()).toBe(false);
  });
});
