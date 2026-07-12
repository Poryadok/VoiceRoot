import { useEffect, useState } from "react";
import { exchangeAuthorizationCode } from "./api";
import { callbackRedirectUri, parseCallbackSearch } from "./callback";
import { setAccessToken, takePkceVerifier } from "./session";

export function OAuthCallback() {
  const [error, setError] = useState("");

  useEffect(() => {
    const params = parseCallbackSearch(window.location.search);
    if (params.error) {
      setError(params.error);
      return;
    }
    if (!params.code) {
      setError("missing_code");
      return;
    }
    const verifier = takePkceVerifier();
    if (!verifier) {
      setError("missing_pkce_verifier");
      return;
    }
    const redirectUri = callbackRedirectUri(window.location.origin);
    exchangeAuthorizationCode({ code: params.code, redirectUri, codeVerifier: verifier })
      .then((token) => {
        setAccessToken(token.access_token);
        window.location.replace("/");
      })
      .catch((err: Error) => setError(err.message));
  }, []);

  return (
    <main className="app-main">
      <h1>Signing in…</h1>
      {error ? <p role="alert">{error}</p> : null}
    </main>
  );
}
