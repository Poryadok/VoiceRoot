import { useCallback, useEffect, useState } from 'react';
import { OAuthCallback } from './OAuthCallback';
import { apiBase, apiFetch, oauthClientId, oauthDisabled } from './oauth/api';
import { buildAuthorizeUrl, callbackRedirectUri, randomCodeVerifier, s256Challenge } from './oauth/pkce';
import { clearSession, getAccessToken, isLoggedIn, setAccessToken, setPkceVerifier, setOAuthState } from './oauth/session';
import { defaultManifest } from './manifestDefaults';
import {
  catalogHasAutocomplete,
  formatCommandLabel,
  parseCommandCatalog,
  type CatalogCommand,
} from './commandCatalog';

type BotSummary = {
  id?: string;
  name?: string;
  description?: string;
  slug?: string;
};

function extractBots(body: Record<string, unknown>): BotSummary[] {
  const list = (body.bot_list as { bots?: BotSummary[] } | undefined)?.bots
    ?? (body.bots as BotSummary[] | undefined)
    ?? [];
  return list.map((row) => {
    const nested = (row as { bot?: BotSummary }).bot;
    return nested ?? row;
  });
}

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
  const [catalog, setCatalog] = useState<CatalogCommand[]>([]);
  const [status, setStatus] = useState('');

  const loadBotCatalog = useCallback(async (botId: string) => {
    if (!botId) {
      setCatalog([]);
      return;
    }
    const [commandsRes, manifestRes] = await Promise.all([
      apiFetch(`/api/v1/bots/${botId}/commands`),
      apiFetch(`/api/v1/bots/${botId}/manifest`),
    ]);
    if (commandsRes.ok) {
      const body = await commandsRes.json();
      const commandsJson = body.command_list?.commands_json ?? body.commands_json ?? '[]';
      try {
        setCatalog(parseCommandCatalog(commandsJson));
      } catch {
        setCatalog([]);
        setStatus('Failed to parse command catalog');
      }
    } else {
      setCatalog([]);
    }
    if (manifestRes.ok) {
      const body = await manifestRes.json();
      const yaml = body.manifest_yaml ?? '';
      if (yaml.trim()) {
        setManifest(yaml);
      }
    }
  }, []);

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
    const list = extractBots(body);
    setBots(list);
    if (list.length > 0 && list[0].id) {
      const nextId = selectedBotId && list.some((b) => b.id === selectedBotId)
        ? selectedBotId
        : list[0].id;
      setSelectedBotId(nextId);
      await loadBotCatalog(nextId);
    }
  }, [loadBotCatalog, selectedBotId]);

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
    setOAuthState(state);
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
    setCatalog([]);
    setManifest(defaultManifest);
    setStatus('Signed out');
  }

  async function selectBot(botId: string) {
    setSelectedBotId(botId);
    setBotToken('');
    setWebhookSecret('');
    setStatus('');
    await loadBotCatalog(botId);
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
    if (id) {
      await loadBotCatalog(id);
    }
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
    if (!res.ok) {
      setStatus(JSON.stringify(body));
      return;
    }
    setStatus('Manifest applied');
    await loadBotCatalog(selectedBotId);
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
                      onClick={() => {
                        if (!bot.id) {
                          return;
                        }
                        void selectBot(bot.id);
                      }}
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

          <section className="catalog" data-testid="command-catalog">
            <h2>Command catalog</h2>
            {!selectedBotId ? (
              <p className="hint">Select a bot to load its registered commands.</p>
            ) : catalog.length === 0 ? (
              <p className="hint">No commands registered — apply a manifest below.</p>
            ) : (
              <>
                {catalogHasAutocomplete(catalog) && (
                  <p className="hint">This bot exposes autocomplete options (shown in the Flutter `/` picker).</p>
                )}
                <ul className="command-catalog-list">
                  {catalog.map((cmd) => (
                    <li key={cmd.name}>
                      <strong>{formatCommandLabel(cmd.name)}</strong>
                      {cmd.description ? <span> — {cmd.description}</span> : null}
                      {cmd.options.length > 0 && (
                        <ul>
                          {cmd.options.map((opt) => (
                            <li key={`${cmd.name}:${opt.name}`}>
                              <code>{opt.name}</code>
                              {opt.type ? ` (${opt.type})` : ''}
                              {opt.required ? ' · required' : ''}
                              {opt.autocomplete ? ' · autocomplete' : ''}
                            </li>
                          ))}
                        </ul>
                      )}
                    </li>
                  ))}
                </ul>
              </>
            )}
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
