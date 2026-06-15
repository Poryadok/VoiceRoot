import { useState } from 'react';

const apiBase = import.meta.env.VITE_VOICE_API_BASE ?? 'http://127.0.0.1:18080';

const defaultManifest = `name: MyBot
description: Example bot
scopes:
  - TEXT_CHAT_SEND_MESSAGES
commands:
  - name: ping
    description: Health check
`;

export function App() {
  const [token, setToken] = useState('');
  const [jwt, setJwt] = useState('');
  const [manifest, setManifest] = useState(defaultManifest);
  const [botId, setBotId] = useState('');
  const [botToken, setBotToken] = useState('');
  const [status, setStatus] = useState('');

  async function registerBot() {
    setStatus('Registering…');
    const res = await fetch(`${apiBase}/api/v1/bots`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${jwt}`,
        'Content-Type': 'application/json',
      },
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
    setBotId(id);
    const regen = await fetch(`${apiBase}/api/v1/bots/${id}/token/regenerate`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${jwt}` },
    });
    const regenBody = await regen.json();
    setBotToken(regenBody.token_response?.token ?? '');
    setStatus(`Registered bot ${id}`);
  }

  async function validateManifest() {
    const res = await fetch(`${apiBase}/api/v1/bots/manifest/validate`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${jwt}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ manifest_yaml: manifest }),
    });
    const body = await res.json();
    setStatus(body.valid ? 'Manifest valid' : (body.errors ?? []).join(', '));
  }

  async function applyManifest() {
    if (!botId) {
      setStatus('Register a bot first');
      return;
    }
    const res = await fetch(`${apiBase}/api/v1/bots/${botId}/manifest`, {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${jwt}`,
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ manifest_yaml: manifest }),
    });
    const body = await res.json();
    setStatus(res.ok ? 'Manifest applied' : JSON.stringify(body));
  }

  return (
    <main className="page">
      <h1>Voice Developer Portal</h1>
      <p className="hint">Phase 16 minimal — register bots, validate/apply manifest, view bot token.</p>

      <label>
        User JWT
        <input value={jwt} onChange={(e) => setJwt(e.target.value)} placeholder="Bearer access token" />
      </label>

      <section>
        <button type="button" onClick={registerBot}>Register bot</button>
        {botId && <p>Bot ID: <code>{botId}</code></p>}
        {botToken && <p>Bot token: <code>{botToken}</code></p>}
      </section>

      <label>
        Manifest YAML
        <textarea rows={12} value={manifest} onChange={(e) => setManifest(e.target.value)} />
      </label>

      <section className="actions">
        <button type="button" onClick={validateManifest}>Validate</button>
        <button type="button" onClick={applyManifest}>Apply to bot</button>
      </section>

      <p className="status">{status}</p>
    </main>
  );
}
