package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
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
class RedisAccountRestoreTokenStoreIntegrationTest {
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
    if (firstConnection != null) {
      firstConnection.destroy();
    }
    if (secondConnection != null) {
      secondConnection.destroy();
    }
  }

  @Test
  void realRedisRepeatedlyAllowsOnlyOneOfTwoStoreInstancesToConsumeAToken() throws Exception {
    RedisAccountRestoreTokenStore first = new RedisAccountRestoreTokenStore(template(firstConnection));
    RedisAccountRestoreTokenStore second = new RedisAccountRestoreTokenStore(template(secondConnection));

    ExecutorService workers = Executors.newFixedThreadPool(2);
    try {
      for (int attempt = 0; attempt < 100; attempt++) {
        UUID accountId = UUID.randomUUID();
        String token = "real-redis-restore-token-" + attempt;
        first.store(token, accountId, Duration.ofMinutes(1));
        CountDownLatch start = new CountDownLatch(1);
        List<Future<java.util.Optional<UUID>>> results =
            List.of(
                workers.submit(() -> consumeWhenStarted(first, token, start)),
                workers.submit(() -> consumeWhenStarted(second, token, start)));
        start.countDown();

        assertThat(results.stream().map(this::await).filter(java.util.Optional::isPresent))
            .containsExactly(java.util.Optional.of(accountId));
      }
    } finally {
      workers.shutdownNow();
    }
  }

  private static java.util.Optional<UUID> consumeWhenStarted(
      RedisAccountRestoreTokenStore store, String token, CountDownLatch start) throws InterruptedException {
    start.await();
    return store.consume(token);
  }

  private static LettuceConnectionFactory connection() {
    LettuceConnectionFactory connection =
        new LettuceConnectionFactory(
            new RedisStandaloneConfiguration(redis.getHost(), redis.getMappedPort(6379)));
    connection.afterPropertiesSet();
    return connection;
  }

  private StringRedisTemplate template(LettuceConnectionFactory connection) {
    StringRedisTemplate template = new StringRedisTemplate(connection);
    template.afterPropertiesSet();
    return template;
  }

  private java.util.Optional<UUID> await(Future<java.util.Optional<UUID>> future) {
    try {
      return future.get();
    } catch (Exception ex) {
      throw new AssertionError(ex);
    }
  }
}
