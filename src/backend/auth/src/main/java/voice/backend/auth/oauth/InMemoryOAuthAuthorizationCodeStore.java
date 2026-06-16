package voice.backend.auth.oauth;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryOAuthAuthorizationCodeStore implements OAuthAuthorizationCodeStore {
  private final Clock clock;
  private final Map<String, OAuthAuthorizationCode> codes = new ConcurrentHashMap<>();

  public InMemoryOAuthAuthorizationCodeStore(Clock clock) {
    this.clock = clock;
  }

  @Override
  public void save(OAuthAuthorizationCode code, Duration ttl) {
    codes.put(code.code(), code);
  }

  @Override
  public Optional<OAuthAuthorizationCode> consume(String code) {
    OAuthAuthorizationCode record = codes.remove(code);
    if (record == null) {
      return Optional.empty();
    }
    if (record.isExpired(Instant.now(clock))) {
      return Optional.empty();
    }
    return Optional.of(record);
  }
}
