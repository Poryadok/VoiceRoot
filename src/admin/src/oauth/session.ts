const SESSION_TOKEN_KEY = "voice_admin_access_token";
const SESSION_PKCE_KEY = "voice_admin_oauth_pkce_verifier";

export function getAccessToken(): string | null {
  return sessionStorage.getItem(SESSION_TOKEN_KEY);
}

export function setAccessToken(token: string): void {
  sessionStorage.setItem(SESSION_TOKEN_KEY, token);
}

export function clearSession(): void {
  sessionStorage.removeItem(SESSION_TOKEN_KEY);
  sessionStorage.removeItem(SESSION_PKCE_KEY);
}

export function setPkceVerifier(verifier: string): void {
  sessionStorage.setItem(SESSION_PKCE_KEY, verifier);
}

export function takePkceVerifier(): string | null {
  const value = sessionStorage.getItem(SESSION_PKCE_KEY);
  sessionStorage.removeItem(SESSION_PKCE_KEY);
  return value;
}

export function isLoggedIn(): boolean {
  return Boolean(getAccessToken());
}
