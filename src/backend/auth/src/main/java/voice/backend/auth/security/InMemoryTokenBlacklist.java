package voice.backend.auth.security;

import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

public class InMemoryTokenBlacklist implements TokenBlacklist {
  private final Clock clock;
  private final Map<String, Instant> revokedUntil = new ConcurrentHashMap<>();

  public InMemoryTokenBlacklist(Clock clock) {
    this.clock = clock;
  }

  @Override
  public void revoke(String jti, Duration ttl) {
    if (jti == null || jti.isBlank() || ttl == null || ttl.isNegative() || ttl.isZero()) {
      return;
    }
    revokedUntil.put(jti, Instant.now(clock).plus(ttl));
  }

  @Override
  public boolean isRevoked(String jti) {
    Instant expiresAt = revokedUntil.get(jti);
    if (expiresAt == null) {
      return false;
    }
    if (!expiresAt.isAfter(Instant.now(clock))) {
      revokedUntil.remove(jti);
      return false;
    }
    return true;
  }

  public Duration ttl(String jti) {
    Instant expiresAt = revokedUntil.get(jti);
    if (expiresAt == null) {
      return Duration.ZERO;
    }
    return Duration.between(Instant.now(clock), expiresAt);
  }
}
