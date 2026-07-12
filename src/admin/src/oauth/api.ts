import { getAccessToken } from "./session";

export function apiBaseUrl(): string {
  return import.meta.env.VITE_VOICE_API_BASE ?? "http://127.0.0.1:18080";
}

export function oauthClientIdValue(): string {
  return import.meta.env.VITE_OAUTH_CLIENT_ID ?? "voice-admin";
}

export function isOauthDisabled(): boolean {
  return import.meta.env.VITE_OAUTH_DISABLED === "true";
}

export async function apiFetch(
  path: string,
  init: RequestInit = {},
): Promise<Response> {
  const headers = new Headers(init.headers);
  const token = getAccessToken();
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  return fetch(`${apiBaseUrl().replace(/\/$/, "")}${path}`, { ...init, headers });
}

export async function exchangeAuthorizationCode(params: {
  code: string;
  redirectUri: string;
  codeVerifier: string;
}): Promise<{ access_token: string; token_type: string; expires_in: number }> {
  const body = new URLSearchParams({
    grant_type: "authorization_code",
    code: params.code,
    redirect_uri: params.redirectUri,
    client_id: oauthClientIdValue(),
    code_verifier: params.codeVerifier,
  });
  const res = await fetch(`${apiBaseUrl().replace(/\/$/, "")}/api/v1/auth/oauth2/token`, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
  });
  const json = await res.json();
  if (!res.ok) {
    throw new Error(json.error ?? "token_exchange_failed");
  }
  return json;
}
