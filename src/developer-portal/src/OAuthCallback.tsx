import { useEffect, useState } from 'react';
import { exchangeAuthorizationCode } from './oauth/api';
import { callbackRedirectUri, parseCallbackSearch } from './oauth/callback';
import { setAccessToken, takePkceVerifier } from './oauth/session';

export function OAuthCallback() {
  const [error, setError] = useState('');

  useEffect(() => {
    const params = parseCallbackSearch(window.location.search);
    if (params.error) {
      setError(params.error);
      return;
    }
    if (!params.code) {
      setError('missing_code');
      return;
    }
    const verifier = takePkceVerifier();
    if (!verifier) {
      setError('missing_pkce_verifier');
      return;
    }
    const redirectUri = callbackRedirectUri(window.location.origin);
    exchangeAuthorizationCode({ code: params.code, redirectUri, codeVerifier: verifier })
      .then((token) => {
        setAccessToken(token.access_token);
        window.location.replace('/');
      })
      .catch((err: Error) => setError(err.message));
  }, []);

  return (
    <main className="page">
      <h1>Signing in…</h1>
      {error && <p className="status error">{error}</p>}
    </main>
  );
}
