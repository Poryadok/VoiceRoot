import { useCallback, useEffect, useState } from 'react';
import { OAuthCallback } from './OAuthCallback';
import { apiFetch, apiBase, oauthClientId, oauthDisabled } from './oauth/api';
import { callbackRedirectUri } from './oauth/callback';
import { buildAuthorizeUrl, randomCodeVerifier, s256Challenge } from './oauth/pkce';
import { clearSession, getAccessToken, isLoggedIn, setAccessToken, setPkceVerifier } from './oauth/session';
import { defaultManifest } from './manifestDefaults';

type BotSummary = {
  id?: string;
  name?: string;
  description?: string;
};

export function App() {
  if (window.location.pathname === '/callback') {
    return <OAuthCallback />;
  }
  return <Portal />;
}

function Portal() {
  const [loggedIn, setLoggedIn] = useState(isLoggedIn());
  const [pasteJwt, setPasteJwt] = useState('');
  const [manifest, setManifest] = useState(defaultManifest);
  const [bots, setBots] = useState<BotSummary[]>([]);
  const [selectedBotId, setSelectedBotId] = useState('');
  const [botToken, setBotToken] = useState('');
  const [webhookSecret, setWebhookSecret] = useState('');
  const [status, setStatus] = useState('');

  const refreshBots = useCallback(async () => {
    if (!isLoggedIn()) {
      return;
    }
    const res = await apiFetch('/api/v1/bots');
    if (!res.ok) {
      setStatus(`List bots failed: ${res.status}`);
      return;
    }
    const body = await res.json();
    const list: BotSummary[] = (body.bots ?? []).map((row: { bot?: BotSummary }) => row.bot ?? row);
    setBots(list);
    if (list.length > 0 && list[0].id) {
      setSelectedBotId(list[0].id);
    }
  }, []);

  useEffect(() => {
    if (loggedIn) {
      void refreshBots();
    }
  }, [loggedIn, refreshBots]);

  async function signInWithVoice() {
    const verifier = randomCodeVerifier();
    const challenge = await s256Challenge(verifier);
    setPkceVerifier(verifier);
    const state = crypto.randomUUID();
    const redirectUri = callbackRedirectUri(window.location.origin);
    const url = buildAuthorizeUrl({
      apiBase,
      clientId: oauthClientId,
      redirectUri,
      state,
      codeChallenge: challenge,
    });
    window.location.assign(url);
  }

  function usePastedJwt() {
    const trimmed = pasteJwt.trim();
    if (!trimmed) {
      setStatus('Paste a JWT first');
      return;
    }
    setAccessToken(trimmed);
    setLoggedIn(true);
    setStatus('Using pasted JWT');
  }

  function logout() {
    clearSession();
    setLoggedIn(false);
    setBots([]);
    setBotToken('');
    setWebhookSecret('');
    setStatus('Signed out');
  }

  async function registerBot() {
    setStatus('Registering…');
    const res = await apiFetch('/api/v1/bots', {
      method: 'POST',
      body: JSON.stringify({
        name: 'DevPortal Bot',
        description: 'Created from developer portal',
        scopes_json: '["TEXT_CHAT_SEND_MESSAGES"]',
      }),
    });
    const body = await res.json();
    if (!res.ok) {
      setStatus(JSON.stringify(body));
      return;
    }
    const id = body.bot?.id ?? '';
    setSelectedBotId(id);
    setBotToken(body.token_response?.token ?? '');
    setWebhookSecret(body.webhook_secret_response?.webhook_secret ?? '');
    setStatus(`Registered bot ${id}`);
    await refreshBots();
  }

  async function revokeAndRegenerateBotToken() {
    if (!selectedBotId) {
      setStatus('Select a bot first');
      return;
    }
    const res = await apiFetch(`/api/v1/bots/${selectedBotId}/token/regenerate`, { method: 'POST' });
    const body = await res.json();
    if (!res.ok) {
      setStatus(JSON.stringify(body));
      return;
    }
    setBotToken(body.token_response?.token ?? '');
    setStatus('Bot token revoked and regenerated');
  }

  async function rotateWebhookSecret() {
    if (!selectedBotId) {
      setStatus('Select a bot first');
      return;
    }
    const res = await apiFetch(`/api/v1/bots/${selectedBotId}/webhook-secret/regenerate`, { method: 'POST' });
    const body = await res.json();
    if (!res.ok) {
      setStatus(JSON.stringify(body));
      return;
    }
    setWebhookSecret(body.webhook_secret_response?.webhook_secret ?? '');
    setStatus('Webhook secret rotated');
  }

  async function validateManifest() {
    const res = await apiFetch('/api/v1/bots/manifest/validate', {
      method: 'POST',
      body: JSON.stringify({ manifest_yaml: manifest }),
    });
    const body = await res.json();
    setStatus(body.valid ? 'Manifest valid' : (body.errors ?? []).join(', '));
  }

  async function applyManifest() {
    if (!selectedBotId) {
      setStatus('Select or register a bot first');
      return;
    }
    const res = await apiFetch(`/api/v1/bots/${selectedBotId}/manifest`, {
      method: 'POST',
      body: JSON.stringify({ manifest_yaml: manifest }),
    });
    const body = await res.json();
    setStatus(res.ok ? 'Manifest applied' : JSON.stringify(body));
  }

  return (
    <main className="page">
      <header className="topbar">
        <h1>Voice Developer Portal</h1>
        {loggedIn ? (
          <button type="button" onClick={logout}>Sign out</button>
        ) : oauthDisabled ? (
          <span className="hint">OAuth disabled (dev paste JWT)</span>
        ) : (
          <button type="button" onClick={() => void signInWithVoice()}>Sign in with Voice</button>
        )}
      </header>

      {!loggedIn && oauthDisabled && (
        <label>
          User JWT (dev only)
          <input value={pasteJwt} onChange={(e) => setPasteJwt(e.target.value)} placeholder="Bearer access token" />
          <button type="button" onClick={usePastedJwt}>Use JWT</button>
        </label>
      )}

      {!loggedIn && !oauthDisabled && (
        <p className="hint">Sign in with your Voice account to manage bots.</p>
      )}

      {loggedIn && (
        <>
          <section>
            <h2>Your bots</h2>
            {bots.length === 0 ? (
              <p className="hint">No bots yet — register one below.</p>
            ) : (
              <ul className="bot-list">
                {bots.map((bot) => (
                  <li key={bot.id}>
                    <button
                      type="button"
                      className={bot.id === selectedBotId ? 'selected' : ''}
                      onClick={() => bot.id && setSelectedBotId(bot.id)}
                    >
                      {bot.name ?? bot.id}
                    </button>
                  </li>
                ))}
              </ul>
            )}
            <button type="button" onClick={() => void registerBot()}>Register bot</button>
            {selectedBotId && (
              <p>Selected bot: <code>{selectedBotId}</code></p>
            )}
            {botToken && <p>Bot token (shown once): <code>{botToken}</code></p>}
            {webhookSecret && <p>Webhook secret (shown once): <code>{webhookSecret}</code></p>}
            <button type="button" disabled={!selectedBotId} onClick={() => void revokeAndRegenerateBotToken()}>
              Revoke &amp; regenerate bot token
            </button>
            <button type="button" disabled={!selectedBotId} onClick={() => void rotateWebhookSecret()}>
              Rotate webhook secret
            </button>
          </section>

          <label>
            Manifest YAML
            <textarea rows={12} value={manifest} onChange={(e) => setManifest(e.target.value)} />
          </label>

          <section className="actions">
            <button type="button" onClick={() => void validateManifest()}>Validate</button>
            <button type="button" onClick={() => void applyManifest()}>Apply to bot</button>
          </section>
        </>
      )}

      <p className="status">{status}</p>
      {loggedIn && !getAccessToken() && <p className="status error">Session expired</p>}
    </main>
  );
}
