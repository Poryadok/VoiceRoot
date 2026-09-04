package voice.backend.auth.sessionepoch;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.time.Duration;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.stream.IntStream;
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
class RedisSessionEpochFloorStoreIntegrationTest {
  @Container
  static final GenericContainer<?> redis =
      new GenericContainer<>(DockerImageName.parse("redis:7-alpine")).withExposedPorts(6379);

  private static LettuceConnectionFactory connectionFactory;
  private static StringRedisTemplate template;

  @BeforeAll
  static void startTemplate() {
    connectionFactory =
        new LettuceConnectionFactory(
            new RedisStandaloneConfiguration(redis.getHost(), redis.getMappedPort(6379)));
    connectionFactory.afterPropertiesSet();
    template = new StringRedisTemplate(connectionFactory);
    template.afterPropertiesSet();
  }

  @AfterAll
  static void closeTemplate() {
    if (connectionFactory != null) {
      connectionFactory.destroy();
    }
  }

  @Test
  void realRedisMaxIsConcurrentMonotonicAndNeverSetsTtl() throws Exception {
    UUID accountId = UUID.randomUUID();
    RedisSessionEpochFloorStore store =
        new RedisSessionEpochFloorStore(new StringRedisSessionEpochCommands(template), Duration.ofSeconds(2));
    ExecutorService workers = Executors.newFixedThreadPool(8);
    try {
      List<Future<Long>> writes =
          IntStream.rangeClosed(1, 16)
              .mapToObj(epoch -> workers.<Long>submit(() -> store.recordAtLeast(accountId, epoch)))
              .toList();

      assertThat(writes.stream().map(this::await).max(Long::compareTo)).hasValue(16L);
      assertThat(store.recordAtLeast(accountId, 4L)).isEqualTo(16L);
      assertThat(store.requireFloor(accountId)).isEqualTo(16L);
      assertThat(template.getExpire(store.keyFor(accountId))).isEqualTo(-1L);
      assertThat(writes).hasSize(16);
    } finally {
      workers.shutdownNow();
    }
  }

  @Test
  void realRedisAtomicCommandRejectsAStaleLowerWriteAfterTheHigherFloorCommitted() {
    UUID accountId = UUID.randomUUID();
    RedisSessionEpochFloorStore store =
        new RedisSessionEpochFloorStore(new StringRedisSessionEpochCommands(template), Duration.ofSeconds(2));

    assertThat(store.recordAtLeast(accountId, 9L)).isEqualTo(9L);
    assertThat(store.recordAtLeast(accountId, 4L)).isEqualTo(9L);
    assertThat(store.requireFloor(accountId)).isEqualTo(9L);
  }

  @Test
  void realRedisPreservesExactInt64FloorsAndFailsClosedForMalformedOrOverflowValues() {
    UUID accountId = UUID.randomUUID();
    RedisSessionEpochFloorStore store =
        new RedisSessionEpochFloorStore(new StringRedisSessionEpochCommands(template), Duration.ofSeconds(2));
    long justAboveDoubleExactRange = 9_007_199_254_740_993L;

    assertThat(store.recordAtLeast(accountId, justAboveDoubleExactRange)).isEqualTo(justAboveDoubleExactRange);
    assertThat(store.recordAtLeast(accountId, justAboveDoubleExactRange - 1L)).isEqualTo(justAboveDoubleExactRange);
    assertThat(store.recordAtLeast(accountId, Long.MAX_VALUE)).isEqualTo(Long.MAX_VALUE);
    assertThat(store.requireFloor(accountId)).isEqualTo(Long.MAX_VALUE);

    UUID malformed = UUID.randomUUID();
    template.opsForValue().set(store.keyFor(malformed), "not-an-int64");
    assertThatThrownBy(() -> store.requireFloor(malformed))
        .isInstanceOf(SessionEpochFloorUnavailableException.class)
        .hasMessageContaining("invalid");

    UUID overflow = UUID.randomUUID();
    template.opsForValue().set(store.keyFor(overflow), "9223372036854775808");
    assertThatThrownBy(() -> store.requireFloor(overflow))
        .isInstanceOf(SessionEpochFloorUnavailableException.class)
        .hasMessageContaining("invalid");
  }

  private long await(Future<Long> future) {
    try {
      return future.get();
    } catch (Exception ex) {
      throw new AssertionError(ex);
    }
  }
}
