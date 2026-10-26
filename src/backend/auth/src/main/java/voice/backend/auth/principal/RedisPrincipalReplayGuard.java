package voice.backend.auth.principal;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.util.HexFormat;
import java.util.Objects;
import java.util.concurrent.*;
import org.springframework.data.redis.core.StringRedisTemplate;

/** Shared replay admission: a single Redis SET NX with the credential's remaining lifetime. */
public final class RedisPrincipalReplayGuard implements AuthPrincipalVerifier.ReplayGuard, AutoCloseable {
  private final StringRedisTemplate redis;
  private final Clock clock;
  private final ExecutorService commands = new ThreadPoolExecutor(
      0, 16, 30, TimeUnit.SECONDS, new SynchronousQueue<>(),
      Thread.ofVirtual().name("auth-principal-replay-", 0).factory(), new ThreadPoolExecutor.AbortPolicy());

  public RedisPrincipalReplayGuard(StringRedisTemplate redis, Clock clock) {
    this.redis = Objects.requireNonNull(redis);
    this.clock = Objects.requireNonNull(clock);
  }

  /** Read-only, bounded startup preflight; it does not consume a credential/JTI. */
  public void requireAvailable() {
    Future<String> pending = null;
    try {
      pending = commands.submit(() -> { try(var connection=redis.getConnectionFactory().getConnection()) { return connection.ping(); } });
      if (!"PONG".equals(pending.get(2,TimeUnit.SECONDS))) throw new PrincipalDependencyUnavailableException();
    } catch (Exception unavailable) {
      if(pending!=null)pending.cancel(true);
      if(unavailable instanceof InterruptedException)Thread.currentThread().interrupt();
      throw new PrincipalDependencyUnavailableException();
    }
  }

  @Override public void record(String issuer, String jti, Instant expires) {
    Future<Boolean> result = null;
    try {
      if (issuer == null || !issuer.matches("[A-Za-z0-9][A-Za-z0-9._-]{0,127}") || jti == null || jti.isBlank()) throw new IllegalArgumentException();
      Duration ttl = Duration.between(clock.instant(), expires);
      if (ttl.isNegative() || ttl.isZero()) throw new IllegalArgumentException();
      String digest = HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(jti.getBytes(StandardCharsets.UTF_8)));
      String key = "auth:principal:replay:" + issuer + ":" + digest;
      result = commands.submit(() -> redis.opsForValue().setIfAbsent(key, "1", ttl));
      Boolean recorded = result.get(2, TimeUnit.SECONDS);
      if (recorded == null) throw new PrincipalDependencyUnavailableException();
      if (!recorded) throw new IllegalArgumentException();
    } catch (Exception ex) {
      if (result != null) result.cancel(true);
      if (ex instanceof InterruptedException) Thread.currentThread().interrupt();
      if (ex instanceof IllegalArgumentException && !(ex instanceof PrincipalDependencyUnavailableException)) {
        throw new IllegalArgumentException("principal replay rejected");
      }
      throw new PrincipalDependencyUnavailableException();
    }
  }

  @Override public void close() { commands.shutdownNow(); }
}
