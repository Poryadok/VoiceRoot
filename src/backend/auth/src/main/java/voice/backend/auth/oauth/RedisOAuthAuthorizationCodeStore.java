package voice.backend.auth.oauth;

import java.time.Duration;
import org.springframework.data.redis.core.StringRedisTemplate;

public class RedisOAuthAuthorizationCodeStore implements OAuthAuthorizationCodeStore {
  private static final String PREFIX = "oauth:code:";

  private final StringRedisTemplate redis;
  private final OAuthAuthorizationCodeCodec codec;

  public RedisOAuthAuthorizationCodeStore(StringRedisTemplate redis, OAuthAuthorizationCodeCodec codec) {
    this.redis = redis;
    this.codec = codec;
  }

  @Override
  public void save(OAuthAuthorizationCode code, Duration ttl) {
    long seconds = Math.max(1L, ttl.toSeconds());
    redis.opsForValue().set(PREFIX + code.code(), codec.encode(code), Duration.ofSeconds(seconds));
  }

  @Override
  public java.util.Optional<OAuthAuthorizationCode> consume(String code) {
    String key = PREFIX + code;
    String raw = redis.opsForValue().getAndDelete(key);
    if (raw == null || raw.isBlank()) {
      return java.util.Optional.empty();
    }
    return java.util.Optional.of(codec.decode(raw));
  }
}
