package voice.backend.auth.oauth;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import java.time.Instant;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.springframework.data.redis.connection.RedisStandaloneConfiguration;
import org.springframework.data.redis.connection.lettuce.LettuceConnectionFactory;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;

@Testcontainers(disabledWithoutDocker = true)
class RedisOAuthAuthorizationCodeStoreIntegrationTest {
  @Container
  static final GenericContainer<?> redis =
      new GenericContainer<>(DockerImageName.parse("redis:7-alpine")).withExposedPorts(6379);

  private static LettuceConnectionFactory firstConnection;
  private static LettuceConnectionFactory secondConnection;

  @BeforeAll
  static void startClients() {
    firstConnection = connection();
    secondConnection = connection();
  }

  @AfterAll
  static void closeClients() {
    if (firstConnection != null) firstConnection.destroy();
    if (secondConnection != null) secondConnection.destroy();
  }

  @Test
  void realRedisPeekIsRepeatableAndFinalConsumeHasOneWinner() throws Exception {
    RedisOAuthAuthorizationCodeStore first = new RedisOAuthAuthorizationCodeStore(template(firstConnection), new OAuthAuthorizationCodeCodec());
    RedisOAuthAuthorizationCodeStore second = new RedisOAuthAuthorizationCodeStore(template(secondConnection), new OAuthAuthorizationCodeCodec());
    OAuthAuthorizationCode code = OAuthAuthorizationCodeStorePeekTest.code("redis-peek-code", Instant.now().plusSeconds(60));
    first.save(code, Duration.ofMinutes(1));
    String key = "oauth:code:" + code.code();
    long beforePeekTtlMillis = template(firstConnection).getExpire(key, TimeUnit.MILLISECONDS);
    assertThat(beforePeekTtlMillis).isBetween(1L, Duration.ofMinutes(2).toMillis());

    assertThat(first.peek(code.code())).contains(code);
    assertThat(second.peek(code.code())).contains(code);
    long afterPeekTtlMillis = template(firstConnection).getExpire(key, TimeUnit.MILLISECONDS);
    assertThat(afterPeekTtlMillis).isGreaterThan(0L).isLessThanOrEqualTo(beforePeekTtlMillis);

    CountDownLatch start = new CountDownLatch(1);
    ExecutorService workers = Executors.newFixedThreadPool(2);
    try {
      List<Future<java.util.Optional<OAuthAuthorizationCode>>> results =
          List.of(workers.submit(() -> consumeWhenStarted(first, code.code(), start)), workers.submit(() -> consumeWhenStarted(second, code.code(), start)));
      start.countDown();
      assertThat(results.stream().map(this::await).filter(java.util.Optional::isPresent))
          .containsExactly(java.util.Optional.of(code));
    } finally {
      workers.shutdownNow();
    }
  }

  @Test
  void realRedisPeekRejectsMissingAndExpiredEntriesWithoutSleep() {
    RedisOAuthAuthorizationCodeStore store = new RedisOAuthAuthorizationCodeStore(template(firstConnection), new OAuthAuthorizationCodeCodec());
    assertThat(store.peek("missing-code")).isEmpty();
    OAuthAuthorizationCode expired = OAuthAuthorizationCodeStorePeekTest.code("redis-expired-code", Instant.now().plusSeconds(60));
    store.save(expired, Duration.ofMinutes(1));
    assertThat(template(firstConnection).expire("oauth:code:" + expired.code(), Duration.ZERO)).isTrue();
    assertThat(store.peek(expired.code())).isEmpty();
    assertThat(store.consume(expired.code())).isEmpty();
  }

  private static java.util.Optional<OAuthAuthorizationCode> consumeWhenStarted(RedisOAuthAuthorizationCodeStore store, String code, CountDownLatch start) throws InterruptedException {
    start.await();
    return store.consume(code);
  }

  private static LettuceConnectionFactory connection() {
    LettuceConnectionFactory connection = new LettuceConnectionFactory(new RedisStandaloneConfiguration(redis.getHost(), redis.getMappedPort(6379)));
    connection.afterPropertiesSet();
    return connection;
  }

  private static StringRedisTemplate template(LettuceConnectionFactory connection) {
    StringRedisTemplate template = new StringRedisTemplate(connection);
    template.afterPropertiesSet();
    return template;
  }

  private java.util.Optional<OAuthAuthorizationCode> await(Future<java.util.Optional<OAuthAuthorizationCode>> future) {
    try { return future.get(); } catch (Exception ex) { throw new AssertionError(ex); }
  }
}
