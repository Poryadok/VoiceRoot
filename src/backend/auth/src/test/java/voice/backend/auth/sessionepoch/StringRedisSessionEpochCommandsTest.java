package voice.backend.auth.sessionepoch;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import io.lettuce.core.RedisFuture;
import io.lettuce.core.TransactionResult;
import io.lettuce.core.api.async.RedisAsyncCommands;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;
import org.mockito.InOrder;
import org.springframework.data.redis.connection.RedisConnection;
import org.springframework.data.redis.connection.RedisConnectionFactory;
import org.springframework.data.redis.core.StringRedisTemplate;

class StringRedisSessionEpochCommandsTest {
  @Test
  void everyCasBorrowsAndClosesItsOwnPipelineBackedConnection() throws Exception {
    StringRedisTemplate template = mock(StringRedisTemplate.class);
    RedisConnectionFactory factory = mock(RedisConnectionFactory.class);
    RedisConnection firstConnection = mock(RedisConnection.class);
    RedisConnection secondConnection = mock(RedisConnection.class);
    RedisAsyncCommands<byte[], byte[]> firstCommands = commandsReturning("1");
    RedisAsyncCommands<byte[], byte[]> secondCommands = commandsReturning("2");
    when(factory.getConnection()).thenReturn(firstConnection, secondConnection);
    when(firstConnection.getNativeConnection()).thenReturn(firstCommands);
    when(secondConnection.getNativeConnection()).thenReturn(secondCommands);
    StringRedisSessionEpochCommands commands = new StringRedisSessionEpochCommands(template, factory);

    assertThat(commands.atomicMaxWithoutExpiry("auth:session:min_epoch:" + UUID.randomUUID(), 1L, Duration.ofSeconds(2)))
        .isEqualTo(1L);
    assertThat(commands.atomicMaxWithoutExpiry("auth:session:min_epoch:" + UUID.randomUUID(), 2L, Duration.ofSeconds(2)))
        .isEqualTo(2L);

    verify(factory, times(2)).getConnection();
    assertPipelineOwnership(firstConnection);
    assertPipelineOwnership(secondConnection);
  }

  private static void assertPipelineOwnership(RedisConnection connection) {
    InOrder calls = inOrder(connection);
    calls.verify(connection).openPipeline();
    calls.verify(connection).getNativeConnection();
    calls.verify(connection).closePipeline();
    calls.verify(connection).close();
  }

  @SuppressWarnings("unchecked")
  private static RedisAsyncCommands<byte[], byte[]> commandsReturning(String storedValue) throws Exception {
    RedisAsyncCommands<byte[], byte[]> commands = mock(RedisAsyncCommands.class);
    TransactionResult result = mock(TransactionResult.class);
    when(result.wasDiscarded()).thenReturn(false);
    when(result.isEmpty()).thenReturn(false);
    when(commands.watch(any(byte[].class))).thenReturn(completed("OK"));
    when(commands.get(any(byte[].class))).thenReturn(completed(storedValue.getBytes(StandardCharsets.UTF_8)));
    when(commands.multi()).thenReturn(completed("OK"));
    when(commands.set(any(byte[].class), any(byte[].class))).thenReturn(completed("OK"));
    when(commands.exec()).thenReturn(completed(result));
    return commands;
  }

  @SuppressWarnings("unchecked")
  private static <T> RedisFuture<T> completed(T value) throws Exception {
    RedisFuture<T> future = mock(RedisFuture.class);
    when(future.get(any(Long.class), any(TimeUnit.class))).thenReturn(value);
    return future;
  }
}
