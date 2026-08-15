const SESSION_TOKEN_KEY = "voice_admin_access_token";
const SESSION_PKCE_KEY = "voice_admin_oauth_pkce_verifier";
const SESSION_OAUTH_STATE_KEY = "voice_admin_oauth_state";
const SESSION_EXPIRES_AT_KEY = "voice_admin_token_expires_at";

export function getAccessToken(): string | null {
  if (!isLoggedIn()) {
    return null;
  }
  return sessionStorage.getItem(SESSION_TOKEN_KEY);
}

export function setAccessToken(token: string, expiresInSeconds?: number): void {
  sessionStorage.setItem(SESSION_TOKEN_KEY, token);
  if (expiresInSeconds && expiresInSeconds > 0) {
    sessionStorage.setItem(
      SESSION_EXPIRES_AT_KEY,
      String(Date.now() + expiresInSeconds * 1000),
    );
  } else {
    sessionStorage.removeItem(SESSION_EXPIRES_AT_KEY);
  }
}

export function clearSession(): void {
  sessionStorage.removeItem(SESSION_TOKEN_KEY);
  sessionStorage.removeItem(SESSION_PKCE_KEY);
  sessionStorage.removeItem(SESSION_OAUTH_STATE_KEY);
  sessionStorage.removeItem(SESSION_EXPIRES_AT_KEY);
}

export function setPkceVerifier(verifier: string): void {
  sessionStorage.setItem(SESSION_PKCE_KEY, verifier);
}

export function takePkceVerifier(): string | null {
  const value = sessionStorage.getItem(SESSION_PKCE_KEY);
  sessionStorage.removeItem(SESSION_PKCE_KEY);
  return value;
}

export function setOAuthState(state: string): void {
  sessionStorage.setItem(SESSION_OAUTH_STATE_KEY, state);
}

export function takeOAuthState(): string | null {
  const value = sessionStorage.getItem(SESSION_OAUTH_STATE_KEY);
  sessionStorage.removeItem(SESSION_OAUTH_STATE_KEY);
  return value;
}

export function isLoggedIn(): boolean {
  const token = sessionStorage.getItem(SESSION_TOKEN_KEY);
  if (!token) {
    return false;
  }
  const expiresAt = sessionStorage.getItem(SESSION_EXPIRES_AT_KEY);
  if (expiresAt && Date.now() >= Number(expiresAt)) {
    clearSession();
    return false;
  }
  return true;
}
