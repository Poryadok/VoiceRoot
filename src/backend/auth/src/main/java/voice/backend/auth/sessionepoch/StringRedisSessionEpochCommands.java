package voice.backend.auth.sessionepoch;

import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import io.lettuce.core.AbstractRedisClient;
import io.lettuce.core.RedisClient;
import io.lettuce.core.RedisFuture;
import io.lettuce.core.TransactionResult;
import io.lettuce.core.api.StatefulRedisConnection;
import io.lettuce.core.api.async.RedisAsyncCommands;
import io.lettuce.core.codec.ByteArrayCodec;
import org.springframework.data.redis.connection.RedisConnection;
import org.springframework.data.redis.connection.RedisConnectionFactory;
import org.springframework.data.redis.connection.lettuce.LettuceConnectionFactory;
import org.springframework.data.redis.core.RedisCallback;
import org.springframework.data.redis.core.StringRedisTemplate;

/**
 * String-only Redis CAS implementation. It deliberately avoids Lua arithmetic because Redis Lua
 * numbers cannot exactly represent every positive signed 64-bit epoch.
 */
final class StringRedisSessionEpochCommands implements RedisSessionEpochCommands {
  private final StringRedisTemplate redis;
  private final RedisClient redisClient;

  StringRedisSessionEpochCommands(StringRedisTemplate redis) {
    if (redis == null) {
      throw new IllegalArgumentException("redis template is required");
    }
    this.redis = redis;
    this.redisClient = standaloneClient(redis.getRequiredConnectionFactory());
  }

  StringRedisSessionEpochCommands(StringRedisTemplate redis, RedisClient redisClient) {
    if (redis == null) {
      throw new IllegalArgumentException("redis template is required");
    }
    if (redisClient == null) {
      throw new IllegalArgumentException("Redis client is required");
    }
    this.redis = redis;
    this.redisClient = redisClient;
  }

  @Override
  public long atomicMaxWithoutExpiry(String key, long candidate, Duration timeout) {
    long deadlineNanos = System.nanoTime() + timeout.toNanos();
    StatefulRedisConnection<byte[], byte[]> connection = redisClient.connect(ByteArrayCodec.INSTANCE);
    try {
      connection.setTimeout(timeout);
      RedisAsyncCommands<byte[], byte[]> commands = connection.async();
      byte[] encodedKey = key.getBytes(StandardCharsets.UTF_8);
      while (System.nanoTime() < deadlineNanos && !Thread.currentThread().isInterrupted()) {
        await(commands.watch(encodedKey), deadlineNanos, connection::close);
        boolean watched = true;
        boolean queued = false;
        try {
          byte[] currentRaw = await(commands.get(encodedKey), deadlineNanos, connection::close);
          long current = currentRaw == null ? 0L : parsePositive(new String(currentRaw, StandardCharsets.UTF_8));
          long next = Math.max(current, candidate);
          await(commands.multi(), deadlineNanos, connection::close);
          queued = true;
          await(
              commands.set(encodedKey, Long.toString(next).getBytes(StandardCharsets.UTF_8)),
              deadlineNanos,
              connection::close);
          TransactionResult result = await(commands.exec(), deadlineNanos, connection::close);
          queued = false;
          watched = false;
          if (!result.wasDiscarded() && !result.isEmpty()) {
            return next;
          }
        } finally {
          if (queued) {
            cancel(commands.discard());
          } else if (watched) {
            cancel(commands.unwatch());
          }
        }
      }
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout");
    } finally {
      if (connection.isOpen()) {
        connection.close();
      }
    }
  }

  @Override
  public long readRequiredPositive(String key, Duration timeout) {
    long deadlineNanos = System.nanoTime() + timeout.toNanos();
    RedisCallback<Long> readFloor =
        connection -> {
          RedisAsyncCommands<byte[], byte[]> commands = commandsFor(connection);
          byte[] raw =
              await(commands.get(key.getBytes(StandardCharsets.UTF_8)), deadlineNanos, connection::close);
          if (raw == null) {
            throw new SessionEpochFloorUnavailableException("session epoch floor missing");
          }
          return parsePositive(new String(raw, StandardCharsets.UTF_8));
        };
    Long value = redis.execute(readFloor);
    if (value == null) {
      throw new SessionEpochFloorUnavailableException("session epoch floor Redis unavailable");
    }
    return value;
  }

  @SuppressWarnings("unchecked")
  private static RedisAsyncCommands<byte[], byte[]> commandsFor(RedisConnection connection) {
    Object nativeConnection = connection.getNativeConnection();
    if (!(nativeConnection instanceof RedisAsyncCommands<?, ?>)) {
      throw new SessionEpochFloorUnavailableException("session epoch floor Redis unavailable");
    }
    return (RedisAsyncCommands<byte[], byte[]>) nativeConnection;
  }

  private static <T> T await(
      RedisFuture<T> future, long deadlineNanos, Runnable closeConnection) {
    long remainingNanos = deadlineNanos - System.nanoTime();
    if (remainingNanos <= 0) {
      abort(future, closeConnection);
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout");
    }
    try {
      return future.get(remainingNanos, TimeUnit.NANOSECONDS);
    } catch (TimeoutException ex) {
      abort(future, closeConnection);
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout", ex);
    } catch (InterruptedException ex) {
      abort(future, closeConnection);
      Thread.currentThread().interrupt();
      throw new SessionEpochFloorUnavailableException("session epoch floor command interrupted", ex);
    } catch (ExecutionException ex) {
      throw new SessionEpochFloorUnavailableException("session epoch floor Redis unavailable", ex.getCause());
    }
  }

  private static void cancel(RedisFuture<?> future) {
    future.cancel(true);
  }

  private static void abort(RedisFuture<?> future, Runnable closeConnection) {
    future.cancel(true);
    closeConnection.run();
  }

  private static RedisClient standaloneClient(RedisConnectionFactory connectionFactory) {
    if (!(connectionFactory instanceof LettuceConnectionFactory lettuceConnectionFactory)) {
      throw new IllegalArgumentException("session epoch floor requires Lettuce Redis");
    }
    AbstractRedisClient client = lettuceConnectionFactory.getRequiredNativeClient();
    if (!(client instanceof RedisClient redisClient)) {
      throw new IllegalArgumentException("session epoch floor requires standalone Redis");
    }
    return redisClient;
  }

  private static long parsePositive(String raw) {
    try {
      long parsed = Long.parseLong(raw);
      if (parsed <= 0) {
        throw new NumberFormatException("non-positive");
      }
      return parsed;
    } catch (NumberFormatException ex) {
      throw new SessionEpochFloorUnavailableException("invalid session epoch floor", ex);
    }
  }
}
