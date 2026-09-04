package voice.backend.auth.sessionepoch;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import io.lettuce.core.RedisFuture;
import io.lettuce.core.RedisClient;
import io.lettuce.core.TransactionResult;
import io.lettuce.core.api.StatefulRedisConnection;
import io.lettuce.core.api.async.RedisAsyncCommands;
import io.lettuce.core.codec.ByteArrayCodec;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import org.junit.jupiter.api.Test;
import org.mockito.InOrder;
import org.springframework.data.redis.core.StringRedisTemplate;

class StringRedisSessionEpochCommandsTest {
  @Test
  void everyCasUsesAndClosesItsOwnPhysicalLettuceConnection() throws Exception {
    StringRedisTemplate template = mock(StringRedisTemplate.class);
    RedisClient client = mock(RedisClient.class);
    StatefulRedisConnection<byte[], byte[]> firstConnection = mock(StatefulRedisConnection.class);
    StatefulRedisConnection<byte[], byte[]> secondConnection = mock(StatefulRedisConnection.class);
    RedisAsyncCommands<byte[], byte[]> firstCommands = commandsReturning("1");
    RedisAsyncCommands<byte[], byte[]> secondCommands = commandsReturning("2");
    when(client.connect(ByteArrayCodec.INSTANCE)).thenReturn(firstConnection, secondConnection);
    when(firstConnection.async()).thenReturn(firstCommands);
    when(secondConnection.async()).thenReturn(secondCommands);
    when(firstConnection.isOpen()).thenReturn(true);
    when(secondConnection.isOpen()).thenReturn(true);
    StringRedisSessionEpochCommands commands = new StringRedisSessionEpochCommands(template, client);

    assertThat(commands.atomicMaxWithoutExpiry("auth:session:min_epoch:" + UUID.randomUUID(), 1L, Duration.ofSeconds(2)))
        .isEqualTo(1L);
    assertThat(commands.atomicMaxWithoutExpiry("auth:session:min_epoch:" + UUID.randomUUID(), 2L, Duration.ofSeconds(2)))
        .isEqualTo(2L);

    verify(client, times(2)).connect(ByteArrayCodec.INSTANCE);
    assertPhysicalConnectionOwnership(firstConnection);
    assertPhysicalConnectionOwnership(secondConnection);
  }

  @Test
  void timedOutCasClosesThePhysicalConnectionExactlyOnce() throws Exception {
    StringRedisTemplate template = mock(StringRedisTemplate.class);
    RedisClient client = mock(RedisClient.class);
    StatefulRedisConnection<byte[], byte[]> connection = mock(StatefulRedisConnection.class);
    RedisAsyncCommands<byte[], byte[]> redis = mock(RedisAsyncCommands.class);
    RedisFuture<String> timedOutWatch = mock(RedisFuture.class);
    when(client.connect(ByteArrayCodec.INSTANCE)).thenReturn(connection);
    when(connection.async()).thenReturn(redis);
    when(timedOutWatch.get(any(Long.class), any(TimeUnit.class))).thenThrow(new TimeoutException("late"));
    when(redis.watch(any(byte[].class))).thenReturn(timedOutWatch);
    StringRedisSessionEpochCommands commands = new StringRedisSessionEpochCommands(template, client);

    assertThatThrownBy(
            () -> commands.atomicMaxWithoutExpiry(
                "auth:session:min_epoch:" + UUID.randomUUID(), 1L, Duration.ofSeconds(2)))
        .isInstanceOf(SessionEpochFloorUnavailableException.class)
        .hasMessageContaining("timeout");

    verify(connection, times(1)).close();
  }

  private static void assertPhysicalConnectionOwnership(
      StatefulRedisConnection<byte[], byte[]> connection) {
    InOrder calls = inOrder(connection);
    calls.verify(connection).async();
    calls.verify(connection).close();
  }

  @SuppressWarnings("unchecked")
  private static RedisAsyncCommands<byte[], byte[]> commandsReturning(String storedValue) throws Exception {
    RedisAsyncCommands<byte[], byte[]> commands = mock(RedisAsyncCommands.class);
    TransactionResult result = mock(TransactionResult.class);
    RedisFuture<String> status = completed("OK");
    RedisFuture<byte[]> current = completed(storedValue.getBytes(StandardCharsets.UTF_8));
    RedisFuture<TransactionResult> executed = completed(result);
    when(result.wasDiscarded()).thenReturn(false);
    when(result.isEmpty()).thenReturn(false);
    when(commands.watch(any(byte[].class))).thenReturn(status);
    when(commands.get(any(byte[].class))).thenReturn(current);
    when(commands.multi()).thenReturn(status);
    when(commands.set(any(byte[].class), any(byte[].class))).thenReturn(status);
    when(commands.exec()).thenReturn(executed);
    return commands;
  }

  @SuppressWarnings("unchecked")
  private static <T> RedisFuture<T> completed(T value) throws Exception {
    RedisFuture<T> future = mock(RedisFuture.class);
    when(future.get(any(Long.class), any(TimeUnit.class))).thenReturn(value);
    return future;
  }
}
