package voice.backend.auth.linkoauth;

/** An exchange is read-only: only authenticated consume can persist a source. */
@FunctionalInterface
public interface LinkedOAuthProvider {
  Result exchange(String provider, String code, String storedCallback, String providerVerifier);
  record Result(String outcome, String externalId, String login, String accessToken, String refreshToken) {
    @Override public String toString() { return "LinkedOAuthResult[redacted]"; }
  }
}
