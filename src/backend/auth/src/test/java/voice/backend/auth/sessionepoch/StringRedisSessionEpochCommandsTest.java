package voice.backend.auth.sessionepoch;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import io.lettuce.core.RedisFuture;
import io.lettuce.core.ScriptOutputType;
import io.lettuce.core.api.async.RedisAsyncCommands;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.springframework.data.redis.connection.RedisConnection;
import org.springframework.data.redis.core.RedisCallback;
import org.springframework.data.redis.core.StringRedisTemplate;

class StringRedisSessionEpochCommandsTest {
  private static final Duration TIMEOUT = Duration.ofSeconds(2);

  @Test
  void atomicMaxUsesOneSharedConnectionEvalWithCanonicalDecimalCandidate() throws Exception {
    StringRedisTemplate template = mock(StringRedisTemplate.class);
    RedisConnection connection = mock(RedisConnection.class);
    RedisAsyncCommands<byte[], byte[]> redis = mock(RedisAsyncCommands.class);
    RedisFuture<Object> result = completed((Object) "9007199254740993".getBytes(StandardCharsets.UTF_8));
    wireSharedConnection(template, connection, redis);
    when(redis.eval(any(byte[].class), eq(ScriptOutputType.VALUE), any(byte[][].class), any(byte[].class)))
        .thenReturn(result);
    StringRedisSessionEpochCommands commands = new StringRedisSessionEpochCommands(template);
    String key = "auth:session:min_epoch:" + UUID.randomUUID();

    assertThat(commands.atomicMaxWithoutExpiry(key, 9_007_199_254_740_993L, TIMEOUT))
        .isEqualTo(9_007_199_254_740_993L);

    ArgumentCaptor<byte[]> script = ArgumentCaptor.forClass(byte[].class);
    ArgumentCaptor<byte[][]> keys = ArgumentCaptor.forClass(byte[][].class);
    ArgumentCaptor<byte[]> candidate = ArgumentCaptor.forClass(byte[].class);
    verify(redis)
        .eval(script.capture(), eq(ScriptOutputType.VALUE), keys.capture(), candidate.capture());
    assertThat(new String(script.getValue(), StandardCharsets.UTF_8)).doesNotContain("tonumber");
    assertThat(keys.getValue().length).isEqualTo(1);
    assertThat(keys.getValue()[0]).isEqualTo(key.getBytes(StandardCharsets.UTF_8));
    assertThat(candidate.getValue())
        .isEqualTo("9007199254740993".getBytes(StandardCharsets.UTF_8));
    verify(connection, never()).close();
  }

  @Test
  void staleWriteRetainsLongMaxValueWithoutFloatingPointConversion() throws Exception {
    StringRedisTemplate template = mock(StringRedisTemplate.class);
    RedisConnection connection = mock(RedisConnection.class);
    RedisAsyncCommands<byte[], byte[]> redis = mock(RedisAsyncCommands.class);
    wireSharedConnection(template, connection, redis);
    RedisFuture<Object> max =
        completed((Object) Long.toString(Long.MAX_VALUE).getBytes(StandardCharsets.UTF_8));
    when(redis.eval(any(byte[].class), eq(ScriptOutputType.VALUE), any(byte[][].class), any(byte[].class)))
        .thenReturn(max);
    StringRedisSessionEpochCommands commands = new StringRedisSessionEpochCommands(template);

    assertThat(commands.atomicMaxWithoutExpiry("auth:session:min_epoch:" + UUID.randomUUID(), 1L, TIMEOUT))
        .isEqualTo(Long.MAX_VALUE);
  }

  @Test
  void timedOutEvalCancelsOnlyItsFutureAndLeavesSharedConnectionOpen() throws Exception {
    StringRedisTemplate template = mock(StringRedisTemplate.class);
    RedisConnection connection = mock(RedisConnection.class);
    RedisAsyncCommands<byte[], byte[]> redis = mock(RedisAsyncCommands.class);
    RedisFuture<Object> timedOut = mock(RedisFuture.class);
    wireSharedConnection(template, connection, redis);
    when(timedOut.get(any(Long.class), any(TimeUnit.class))).thenThrow(new TimeoutException("late"));
    when(redis.eval(any(byte[].class), eq(ScriptOutputType.VALUE), any(byte[][].class), any(byte[].class)))
        .thenReturn(timedOut);
    StringRedisSessionEpochCommands commands = new StringRedisSessionEpochCommands(template);

    assertThatThrownBy(
            () ->
                commands.atomicMaxWithoutExpiry(
                    "auth:session:min_epoch:" + UUID.randomUUID(), 1L, TIMEOUT))
        .isInstanceOf(SessionEpochFloorUnavailableException.class)
        .hasMessageContaining("timeout");

    verify(timedOut).cancel(true);
    verify(connection, never()).close();
  }

  @Test
  void readUsesSharedConnectionAndRejectsOverflow() throws Exception {
    StringRedisTemplate template = mock(StringRedisTemplate.class);
    RedisConnection connection = mock(RedisConnection.class);
    RedisAsyncCommands<byte[], byte[]> redis = mock(RedisAsyncCommands.class);
    wireSharedConnection(template, connection, redis);
    RedisFuture<byte[]> overflow = completed("9223372036854775808".getBytes(StandardCharsets.UTF_8));
    when(redis.get(any(byte[].class))).thenReturn(overflow);
    StringRedisSessionEpochCommands commands = new StringRedisSessionEpochCommands(template);

    assertThatThrownBy(
            () -> commands.readRequiredPositive("auth:session:min_epoch:" + UUID.randomUUID(), TIMEOUT))
        .isInstanceOf(SessionEpochFloorUnavailableException.class)
        .hasMessageContaining("invalid");
    verify(connection, never()).close();
  }

  @SuppressWarnings("unchecked")
  private static void wireSharedConnection(
      StringRedisTemplate template,
      RedisConnection connection,
      RedisAsyncCommands<byte[], byte[]> redis) {
    when(template.execute(any(RedisCallback.class)))
        .thenAnswer(invocation -> ((RedisCallback<Object>) invocation.getArgument(0)).doInRedis(connection));
    when(connection.getNativeConnection()).thenReturn(redis);
  }

  @SuppressWarnings("unchecked")
  private static <T> RedisFuture<T> completed(T value) throws Exception {
    RedisFuture<T> future = mock(RedisFuture.class);
    when(future.get(any(Long.class), any(TimeUnit.class))).thenReturn(value);
    return future;
  }
}
