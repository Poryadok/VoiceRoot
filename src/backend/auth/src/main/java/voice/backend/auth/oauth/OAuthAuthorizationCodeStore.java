package voice.backend.auth.oauth;

import java.time.Duration;
import java.util.Optional;

public interface OAuthAuthorizationCodeStore {
  void save(OAuthAuthorizationCode code, Duration ttl);

  /** Returns the code record and removes it (one-time use). */
  Optional<OAuthAuthorizationCode> consume(String code);
}
