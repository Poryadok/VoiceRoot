package voice.backend.auth.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import java.time.Duration;
import java.util.List;
import java.util.UUID;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.ValueOperations;

class RedisAccountRestoreTokenStoreTest {
  private static final String PREFIX = "account:restore:";

  @Test
  void deterministicSplitReadHarnessAllowsOnlyOneConcurrentConsumeWinner() throws Exception {
    String token = "restore-token-plaintext-must-not-be-a-key";
    UUID accountId = UUID.randomUUID();
    AtomicReference<String> storedValue = new AtomicReference<>(accountId.toString());
    CyclicBarrier simultaneousReads = new CyclicBarrier(2);

    ValueOperations<String, String> firstValues = valuesWithSplitReadDelete(storedValue, simultaneousReads);
    ValueOperations<String, String> secondValues = valuesWithSplitReadDelete(storedValue, simultaneousReads);
    StringRedisTemplate firstRedis = redisWith(firstValues);
    StringRedisTemplate secondRedis = redisWith(secondValues);
    RedisAccountRestoreTokenStore first = new RedisAccountRestoreTokenStore(firstRedis);
    RedisAccountRestoreTokenStore second = new RedisAccountRestoreTokenStore(secondRedis);

    ExecutorService workers = Executors.newFixedThreadPool(2);
    try {
      List<Future<java.util.Optional<UUID>>> results =
          List.of(workers.submit(() -> first.consume(token)), workers.submit(() -> second.consume(token)));

      assertThat(results.stream().map(this::await).filter(java.util.Optional::isPresent)).hasSize(1);
    } finally {
      workers.shutdownNow();
    }
  }

  @Test
  void consumeUsesOneAtomicRedisGetAndDelete() {
    String token = "one-time-restore-token";
    UUID accountId = UUID.randomUUID();
    String key = PREFIX + AccountRestoreTokenHash.of(token);
    ValueOperations<String, String> values = mock();
    when(values.get(key)).thenReturn(accountId.toString());
    when(values.getAndDelete(key)).thenReturn(accountId.toString());
    StringRedisTemplate redis = redisWith(values);

    assertThat(new RedisAccountRestoreTokenStore(redis).consume(token)).contains(accountId);

    verify(values).getAndDelete(key);
    verify(values, never()).get(any());
    verify(redis, never()).delete(anyString());
  }

  @Test
  void storePersistsOnlyTheHashAsTheRedisKey() {
    String token = "restore-token-plaintext-must-not-be-persisted";
    UUID accountId = UUID.randomUUID();
    ValueOperations<String, String> values = mock();
    StringRedisTemplate redis = redisWith(values);

    new RedisAccountRestoreTokenStore(redis).store(token, accountId, Duration.ofDays(30));

    org.mockito.ArgumentCaptor<String> key = org.mockito.ArgumentCaptor.forClass(String.class);
    org.mockito.ArgumentCaptor<String> value = org.mockito.ArgumentCaptor.forClass(String.class);
    verify(values).set(key.capture(), value.capture(), eq(Duration.ofDays(30)));
    assertThat(key.getValue()).isEqualTo(PREFIX + AccountRestoreTokenHash.of(token));
    assertThat(key.getValue()).doesNotContain(token);
    assertThat(value.getValue()).doesNotContain(token);
  }

  private ValueOperations<String, String> valuesWithSplitReadDelete(
      AtomicReference<String> storedValue, CyclicBarrier simultaneousReads) throws Exception {
    ValueOperations<String, String> values = mock();
    when(values.get(any())).thenAnswer(
        ignored -> {
          String observed = storedValue.get();
          simultaneousReads.await();
          return observed;
        });
    when(values.getAndDelete(any())).thenAnswer(ignored -> storedValue.getAndSet(null));
    return values;
  }

  private StringRedisTemplate redisWith(ValueOperations<String, String> values) {
    StringRedisTemplate redis = mock();
    when(redis.opsForValue()).thenReturn(values);
    return redis;
  }

  private java.util.Optional<UUID> await(Future<java.util.Optional<UUID>> future) {
    try {
      return future.get();
    } catch (Exception ex) {
      throw new AssertionError(ex);
    }
  }
}
