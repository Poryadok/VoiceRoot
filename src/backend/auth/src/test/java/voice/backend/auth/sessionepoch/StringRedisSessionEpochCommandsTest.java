package voice.backend.auth.sessionepoch;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import io.lettuce.core.ConnectionFuture;
import io.lettuce.core.RedisFuture;
import io.lettuce.core.RedisClient;
import io.lettuce.core.RedisURI;
import io.lettuce.core.TransactionResult;
import io.lettuce.core.api.StatefulRedisConnection;
import io.lettuce.core.api.async.RedisAsyncCommands;
import io.lettuce.core.codec.ByteArrayCodec;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import java.util.function.BiConsumer;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;
import org.springframework.data.redis.core.StringRedisTemplate;

class StringRedisSessionEpochCommandsTest {
  private static final RedisURI ENDPOINT = RedisURI.create("redis://localhost:6379/0");

  @Test
  void everyCasUsesAndClosesItsOwnPhysicalLettuceConnection() throws Exception {
    StringRedisTemplate template = mock(StringRedisTemplate.class);
    RedisClient client = mock(RedisClient.class);
    StatefulRedisConnection<byte[], byte[]> firstConnection = mock(StatefulRedisConnection.class);
    StatefulRedisConnection<byte[], byte[]> secondConnection = mock(StatefulRedisConnection.class);
    RedisAsyncCommands<byte[], byte[]> firstCommands = commandsReturning("1");
    RedisAsyncCommands<byte[], byte[]> secondCommands = commandsReturning("2");
    ConnectionFuture<StatefulRedisConnection<byte[], byte[]>> firstConnecting = connected(firstConnection);
    ConnectionFuture<StatefulRedisConnection<byte[], byte[]>> secondConnecting = connected(secondConnection);
    when(client.connectAsync(ByteArrayCodec.INSTANCE, ENDPOINT))
        .thenReturn(firstConnecting, secondConnecting);
    when(firstConnection.async()).thenReturn(firstCommands);
    when(secondConnection.async()).thenReturn(secondCommands);
    when(firstConnection.isOpen()).thenReturn(true);
    when(secondConnection.isOpen()).thenReturn(true);
    StringRedisSessionEpochCommands commands = new StringRedisSessionEpochCommands(template, client, ENDPOINT);

    assertThat(commands.atomicMaxWithoutExpiry("auth:session:min_epoch:" + UUID.randomUUID(), 1L, Duration.ofSeconds(2)))
        .isEqualTo(1L);
    assertThat(commands.atomicMaxWithoutExpiry("auth:session:min_epoch:" + UUID.randomUUID(), 2L, Duration.ofSeconds(2)))
        .isEqualTo(2L);

    verify(client, times(2)).connectAsync(ByteArrayCodec.INSTANCE, ENDPOINT);
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
    ConnectionFuture<StatefulRedisConnection<byte[], byte[]>> connecting = connected(connection);
    when(client.connectAsync(ByteArrayCodec.INSTANCE, ENDPOINT)).thenReturn(connecting);
    when(connection.async()).thenReturn(redis);
    when(timedOutWatch.get(any(Long.class), any(TimeUnit.class))).thenThrow(new TimeoutException("late"));
    when(redis.watch(any(byte[].class))).thenReturn(timedOutWatch);
    StringRedisSessionEpochCommands commands = new StringRedisSessionEpochCommands(template, client, ENDPOINT);

    assertThatThrownBy(
            () -> commands.atomicMaxWithoutExpiry(
                "auth:session:min_epoch:" + UUID.randomUUID(), 1L, Duration.ofSeconds(2)))
        .isInstanceOf(SessionEpochFloorUnavailableException.class)
        .hasMessageContaining("timeout");

    verify(connection, times(1)).closeAsync();
  }

  @Test
  void readUsesAndClosesItsOwnPhysicalConnection() throws Exception {
    StringRedisTemplate template = mock(StringRedisTemplate.class);
    RedisClient client = mock(RedisClient.class);
    StatefulRedisConnection<byte[], byte[]> connection = mock(StatefulRedisConnection.class);
    RedisAsyncCommands<byte[], byte[]> redis = mock(RedisAsyncCommands.class);
    RedisFuture<byte[]> value = completed("9007199254740993".getBytes(StandardCharsets.UTF_8));
    ConnectionFuture<StatefulRedisConnection<byte[], byte[]>> connecting = connected(connection);
    when(client.connectAsync(ByteArrayCodec.INSTANCE, ENDPOINT)).thenReturn(connecting);
    when(connection.async()).thenReturn(redis);
    when(connection.isOpen()).thenReturn(true);
    when(redis.get(any(byte[].class))).thenReturn(value);
    StringRedisSessionEpochCommands commands = new StringRedisSessionEpochCommands(template, client, ENDPOINT);

    assertThat(commands.readRequiredPositive("auth:session:min_epoch:" + UUID.randomUUID(), Duration.ofSeconds(2)))
        .isEqualTo(9_007_199_254_740_993L);

    verify(client).connectAsync(ByteArrayCodec.INSTANCE, ENDPOINT);
    assertPhysicalConnectionOwnership(connection);
  }

  @Test
  void timedOutReadClosesThePhysicalConnectionExactlyOnce() throws Exception {
    StringRedisTemplate template = mock(StringRedisTemplate.class);
    RedisClient client = mock(RedisClient.class);
    StatefulRedisConnection<byte[], byte[]> connection = mock(StatefulRedisConnection.class);
    RedisAsyncCommands<byte[], byte[]> redis = mock(RedisAsyncCommands.class);
    RedisFuture<byte[]> timedOutRead = mock(RedisFuture.class);
    ConnectionFuture<StatefulRedisConnection<byte[], byte[]>> connecting = connected(connection);
    when(client.connectAsync(ByteArrayCodec.INSTANCE, ENDPOINT)).thenReturn(connecting);
    when(connection.async()).thenReturn(redis);
    when(timedOutRead.get(any(Long.class), any(TimeUnit.class))).thenThrow(new TimeoutException("late"));
    when(redis.get(any(byte[].class))).thenReturn(timedOutRead);
    StringRedisSessionEpochCommands commands = new StringRedisSessionEpochCommands(template, client, ENDPOINT);

    assertThatThrownBy(
            () -> commands.readRequiredPositive(
                "auth:session:min_epoch:" + UUID.randomUUID(), Duration.ofSeconds(2)))
        .isInstanceOf(SessionEpochFloorUnavailableException.class)
        .hasMessageContaining("timeout");

    verify(connection, times(1)).closeAsync();
  }

  @Test
  void timedOutConnectionKeepsItsFutureObservableAndClosesAnyLatePhysicalConnection()
      throws Exception {
    StringRedisTemplate template = mock(StringRedisTemplate.class);
    RedisClient client = mock(RedisClient.class);
    ConnectionFuture<StatefulRedisConnection<byte[], byte[]>> connecting = mock(ConnectionFuture.class);
    StatefulRedisConnection<byte[], byte[]> lateConnection = mock(StatefulRedisConnection.class);
    when(client.connectAsync(ByteArrayCodec.INSTANCE, ENDPOINT)).thenReturn(connecting);
    when(connecting.get(any(Long.class), any(TimeUnit.class))).thenThrow(new TimeoutException("late"));
    StringRedisSessionEpochCommands commands = new StringRedisSessionEpochCommands(template, client, ENDPOINT);

    assertThatThrownBy(
            () -> commands.readRequiredPositive(
                "auth:session:min_epoch:" + UUID.randomUUID(), Duration.ofSeconds(2)))
        .isInstanceOf(SessionEpochFloorUnavailableException.class)
        .hasMessageContaining("timeout");

    verify(connecting, never()).cancel(true);
    ArgumentCaptor<BiConsumer<StatefulRedisConnection<byte[], byte[]>, Throwable>> completion =
        ArgumentCaptor.forClass(BiConsumer.class);
    verify(connecting).whenComplete(completion.capture());
    completion.getValue().accept(lateConnection, null);
    verify(lateConnection).closeAsync();
  }

  private static void assertPhysicalConnectionOwnership(
      StatefulRedisConnection<byte[], byte[]> connection) {
    InOrder calls = inOrder(connection);
    calls.verify(connection).async();
    calls.verify(connection).closeAsync();
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

  @SuppressWarnings("unchecked")
  private static <T> ConnectionFuture<T> connected(T value) throws Exception {
    ConnectionFuture<T> future = mock(ConnectionFuture.class);
    when(future.get(any(Long.class), any(TimeUnit.class))).thenReturn(value);
    return future;
  }
}
