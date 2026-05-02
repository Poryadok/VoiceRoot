package voice.backend.auth.security;

import java.time.Duration;
import org.springframework.data.redis.core.StringRedisTemplate;

public class RedisTokenBlacklist implements TokenBlacklist {
  private final StringRedisTemplate redis;
  private final String keyPrefix;

  public RedisTokenBlacklist(StringRedisTemplate redis, String keyPrefix) {
    this.redis = redis;
    this.keyPrefix = keyPrefix == null || keyPrefix.isBlank() ? "jwt:blacklist:" : keyPrefix;
  }

  @Override
  public void revoke(String jti, Duration ttl) {
    if (jti == null || jti.isBlank() || ttl == null || ttl.isNegative() || ttl.isZero()) {
      return;
    }
    long seconds = Math.max(1L, ttl.toSeconds());
    redis.opsForValue().set(keyPrefix + jti, "1", Duration.ofSeconds(seconds));
  }

  @Override
  public boolean isRevoked(String jti) {
    return Boolean.TRUE.equals(redis.hasKey(keyPrefix + jti));
  }
}
