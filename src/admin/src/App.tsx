import { useCallback, useEffect, useState } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";
import { AuditPage } from "./pages/AuditPage";
import { AnalyticsExportPage } from "./pages/AnalyticsExportPage";
import { FunnelsPage } from "./pages/FunnelsPage";
import { ProductAnalyticsPage } from "./pages/ProductAnalyticsPage";
import { QueuePage } from "./pages/QueuePage";
import { OAuthCallback } from "./oauth/OAuthCallback";
import { apiBaseUrl, isOauthDisabled, oauthClientIdValue } from "./oauth/api";
import { callbackRedirectUri } from "./oauth/callback";
import { buildAuthorizeUrl, randomCodeVerifier, s256Challenge } from "./oauth/pkce";
import {
  clearSession,
  getAccessToken,
  isLoggedIn,
  setAccessToken,
  setPkceVerifier,
} from "./oauth/session";

function LoginScreen() {
  const [pasteJwt, setPasteJwt] = useState("");
  const [status, setStatus] = useState("");

  const signInWithVoice = useCallback(async () => {
    const verifier = randomCodeVerifier();
    const challenge = await s256Challenge(verifier);
    setPkceVerifier(verifier);
    const state = crypto.randomUUID();
    const redirectUri = callbackRedirectUri(window.location.origin);
    window.location.href = buildAuthorizeUrl({
      apiBase: apiBaseUrl(),
      clientId: oauthClientIdValue(),
      redirectUri,
      state,
      codeChallenge: challenge,
    });
  }, []);

  function applyPasteJwt() {
    const token = pasteJwt.trim();
    if (!token) {
      setStatus("Paste a staff JWT first.");
      return;
    }
    setAccessToken(token);
    window.location.reload();
  }

  return (
    <main className="app-main">
      <h1>Voice Admin</h1>
      <p>Sign in with your Voice staff account.</p>
      {!isOauthDisabled() ? (
        <button type="button" onClick={() => void signInWithVoice()}>
          Sign in with Voice
        </button>
      ) : (
        <>
          <label>
            Staff JWT
            <input
              type="text"
              value={pasteJwt}
              onChange={(e) => setPasteJwt(e.target.value)}
              autoComplete="off"
            />
          </label>
          <button type="button" onClick={applyPasteJwt}>
            Continue
          </button>
        </>
      )}
      {status ? <p role="status">{status}</p> : null}
    </main>
  );
}

function AdminShell() {
  const [loggedIn, setLoggedIn] = useState(isLoggedIn());

  useEffect(() => {
    setLoggedIn(Boolean(getAccessToken()));
  }, []);

  if (!loggedIn) {
    return <LoginScreen />;
  }

  return (
    <Layout onSignOut={() => { clearSession(); setLoggedIn(false); }}>
      <Routes>
        <Route path="/" element={<Navigate to="/queue" replace />} />
        <Route path="/queue" element={<QueuePage />} />
        <Route path="/audit" element={<AuditPage />} />
        <Route path="/analytics/product" element={<ProductAnalyticsPage />} />
        <Route path="/analytics/funnels" element={<FunnelsPage />} />
        <Route path="/analytics/export" element={<AnalyticsExportPage />} />
      </Routes>
    </Layout>
  );
}

export function App() {
  return (
    <Routes>
      <Route path="/callback" element={<OAuthCallback />} />
      <Route path="/*" element={<AdminShell />} />
    </Routes>
  );
}
