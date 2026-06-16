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
    Instant expiresAt) {

  public boolean isExpired(Instant now) {
    return !expiresAt.isAfter(now);
  }
}
