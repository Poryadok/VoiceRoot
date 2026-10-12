package voice.backend.auth.oauth;

import java.time.Instant;
import java.util.Optional;

public record OAuthAuthorizationCode(
    String code,
    String accountId,
    String profileId,
    String clientId,
    String redirectUri,
    String codeChallenge,
    String codeChallengeMethod,
    Instant expiresAt,
    long originSessionEpoch) {

  /** Legacy stored payloads have no origin epoch and are deliberately unbound. */
  public OAuthAuthorizationCode(
      String code,
      String accountId,
      String profileId,
      String clientId,
      String redirectUri,
      String codeChallenge,
      String codeChallengeMethod,
      Instant expiresAt) {
    this(code, accountId, profileId, clientId, redirectUri, codeChallenge,
        codeChallengeMethod, expiresAt, 0);
  }

  public boolean isExpired(Instant now) {
    return !expiresAt.isAfter(now);
  }
}
