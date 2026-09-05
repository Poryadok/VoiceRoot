package voice.backend.auth.service;

import java.time.Duration;
import java.util.Optional;
import java.util.UUID;
import org.springframework.data.redis.core.StringRedisTemplate;

public class RedisAccountRestoreTokenStore implements AccountRestoreTokenStore {
  private static final String PREFIX = "account:restore:";

  private final StringRedisTemplate redis;

  public RedisAccountRestoreTokenStore(StringRedisTemplate redis) {
    this.redis = redis;
  }

  @Override
  public void store(String token, UUID accountId, Duration ttl) {
    redis.opsForValue().set(PREFIX + AccountRestoreTokenHash.of(token), accountId.toString(), ttl);
  }

  @Override
  public Optional<UUID> consume(String token) {
    String key = PREFIX + AccountRestoreTokenHash.of(token);
    String accountId = redis.opsForValue().get(key);
    if (accountId == null) {
      return Optional.empty();
    }
    redis.delete(key);
    return Optional.of(UUID.fromString(accountId));
  }
}
