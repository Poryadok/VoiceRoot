import { describe, expect, it } from 'vitest';
import { randomCodeVerifier, s256Challenge } from '../oauth/pkce';
import { callbackRedirectUri, parseCallbackSearch } from '../oauth/callback';

describe('PKCE helpers', () => {
  it('generates verifier and matching S256 challenge', async () => {
    const verifier = randomCodeVerifier();
    expect(verifier.length).toBeGreaterThan(40);
    const challenge = await s256Challenge(verifier);
    expect(challenge).toMatch(/^[A-Za-z0-9_-]+$/);
    const repeat = await s256Challenge(verifier);
    expect(repeat).toBe(challenge);
  });
});

describe('callback URL parser', () => {
  it('extracts code and state', () => {
    const parsed = parseCallbackSearch('?code=abc123&state=xyz');
    expect(parsed.code).toBe('abc123');
    expect(parsed.state).toBe('xyz');
    expect(parsed.error).toBeNull();
  });

  it('reads oauth error param', () => {
    const parsed = parseCallbackSearch('?error=access_denied&state=s');
    expect(parsed.error).toBe('access_denied');
    expect(parsed.code).toBeNull();
  });

  it('builds callback redirect uri from origin', () => {
    expect(callbackRedirectUri('http://localhost:9082')).toBe('http://localhost:9082/callback');
    expect(callbackRedirectUri('http://localhost:9082/')).toBe('http://localhost:9082/callback');
  });
});
