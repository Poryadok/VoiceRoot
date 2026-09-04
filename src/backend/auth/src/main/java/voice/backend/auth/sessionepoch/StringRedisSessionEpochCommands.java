package voice.backend.auth.sessionepoch;

import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import io.lettuce.core.RedisFuture;
import io.lettuce.core.TransactionResult;
import io.lettuce.core.api.async.RedisAsyncCommands;
import org.springframework.data.redis.connection.RedisConnection;
import org.springframework.data.redis.core.RedisCallback;
import org.springframework.data.redis.core.StringRedisTemplate;

/**
 * String-only Redis CAS implementation. It deliberately avoids Lua arithmetic because Redis Lua
 * numbers cannot exactly represent every positive signed 64-bit epoch.
 */
final class StringRedisSessionEpochCommands implements RedisSessionEpochCommands {
  private final StringRedisTemplate redis;

  StringRedisSessionEpochCommands(StringRedisTemplate redis) {
    if (redis == null) {
      throw new IllegalArgumentException("redis template is required");
    }
    this.redis = redis;
  }

  @Override
  public long atomicMaxWithoutExpiry(String key, long candidate, Duration timeout) {
    long deadlineNanos = System.nanoTime() + timeout.toNanos();
    Long recorded =
        redis.execute(
            new RedisCallback<>() {
              @Override
              public Long doInRedis(RedisConnection connection) {
                RedisAsyncCommands<byte[], byte[]> commands = commandsFor(connection);
                byte[] encodedKey = key.getBytes(StandardCharsets.UTF_8);
                while (System.nanoTime() < deadlineNanos && !Thread.currentThread().isInterrupted()) {
                  await(commands.watch(encodedKey), deadlineNanos, connection);
                  boolean watched = true;
                  boolean queued = false;
                  try {
                    byte[] currentRaw = await(commands.get(encodedKey), deadlineNanos, connection);
                    long current = currentRaw == null ? 0L : parsePositive(new String(currentRaw, StandardCharsets.UTF_8));
                    long next = Math.max(current, candidate);
                    await(commands.multi(), deadlineNanos, connection);
                    queued = true;
                    await(
                        commands.set(encodedKey, Long.toString(next).getBytes(StandardCharsets.UTF_8)),
                        deadlineNanos,
                        connection);
                    TransactionResult result = await(commands.exec(), deadlineNanos, connection);
                    queued = false;
                    watched = false;
                    if (!result.wasDiscarded() && !result.isEmpty()) {
                      return next;
                    }
                  } finally {
                    if (queued) {
                      cancel(commands.discard(), connection);
                    } else if (watched) {
                      cancel(commands.unwatch(), connection);
                    }
                  }
                }
                throw new SessionEpochFloorUnavailableException("session epoch floor command timeout");
              }
            });
    if (recorded == null) {
      throw new SessionEpochFloorUnavailableException("session epoch floor Redis unavailable");
    }
    return recorded;
  }

  @Override
  public long readRequiredPositive(String key, Duration timeout) {
    long deadlineNanos = System.nanoTime() + timeout.toNanos();
    RedisCallback<Long> readFloor =
        connection -> {
          RedisAsyncCommands<byte[], byte[]> commands = commandsFor(connection);
          byte[] raw =
              await(commands.get(key.getBytes(StandardCharsets.UTF_8)), deadlineNanos, connection);
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
      RedisFuture<T> future, long deadlineNanos, RedisConnection connection) {
    long remainingNanos = deadlineNanos - System.nanoTime();
    if (remainingNanos <= 0) {
      abort(future, connection);
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout");
    }
    try {
      return future.get(remainingNanos, TimeUnit.NANOSECONDS);
    } catch (TimeoutException ex) {
      abort(future, connection);
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout", ex);
    } catch (InterruptedException ex) {
      abort(future, connection);
      Thread.currentThread().interrupt();
      throw new SessionEpochFloorUnavailableException("session epoch floor command interrupted", ex);
    } catch (ExecutionException ex) {
      throw new SessionEpochFloorUnavailableException("session epoch floor Redis unavailable", ex.getCause());
    }
  }

  private static void cancel(RedisFuture<?> future, RedisConnection connection) {
    try {
      future.cancel(true);
    } catch (RuntimeException ignored) {
      connection.close();
    }
  }

  private static void abort(RedisFuture<?> future, RedisConnection connection) {
    future.cancel(true);
    connection.close();
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
