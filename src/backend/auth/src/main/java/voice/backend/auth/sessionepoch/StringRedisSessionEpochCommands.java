package voice.backend.auth.sessionepoch;

import io.lettuce.core.RedisFuture;
import io.lettuce.core.ScriptOutputType;
import io.lettuce.core.cluster.api.async.RedisClusterAsyncCommands;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import org.springframework.data.redis.connection.RedisConnection;
import org.springframework.data.redis.core.RedisCallback;
import org.springframework.data.redis.core.StringRedisTemplate;

/**
 * String-only Redis CAS implementation. Lua compares canonical decimal strings rather than Lua
 * numbers, which cannot exactly represent every positive signed 64-bit epoch.
 */
final class StringRedisSessionEpochCommands implements RedisSessionEpochCommands {
  private static final byte[] ATOMIC_MAX_SCRIPT =
      ("local current = redis.call('GET', KEYS[1])\n"
              + "local candidate = ARGV[1]\n"
              + "if not current then\n"
              + "  redis.call('SET', KEYS[1], candidate)\n"
              + "  return candidate\n"
              + "end\n"
              + "if not string.match(current, '^[1-9][0-9]*$')\n"
              + "   or #current > 19\n"
              + "   or (#current == 19 and current > '9223372036854775807') then\n"
              + "  return redis.error_reply('invalid session epoch floor')\n"
              + "end\n"
              + "if #current > #candidate or (#current == #candidate and current >= candidate) then\n"
              + "  redis.call('SET', KEYS[1], current)\n"
              + "  return current\n"
              + "end\n"
              + "redis.call('SET', KEYS[1], candidate)\n"
              + "return candidate\n")
          .getBytes(StandardCharsets.UTF_8);

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
    byte[] encodedKey = key.getBytes(StandardCharsets.UTF_8);
    byte[] encodedCandidate = Long.toString(candidate).getBytes(StandardCharsets.UTF_8);
    return redis.execute(
        (RedisCallback<Long>)
        connection -> {
          RedisFuture<Object> future =
              commandsFor(connection)
                  .eval(
                      ATOMIC_MAX_SCRIPT,
                      ScriptOutputType.VALUE,
                      new byte[][] {encodedKey},
                      encodedCandidate);
          Object recorded = await(future, deadlineNanos);
          if (!(recorded instanceof byte[] raw)) {
            throw new SessionEpochFloorUnavailableException("invalid session epoch floor");
          }
          return parsePositive(new String(raw, StandardCharsets.UTF_8));
        });
  }

  @Override
  public long readRequiredPositive(String key, Duration timeout) {
    long deadlineNanos = System.nanoTime() + timeout.toNanos();
    return redis.execute(
        (RedisCallback<Long>)
        connection -> {
          byte[] raw =
              await(
                  commandsFor(connection).get(key.getBytes(StandardCharsets.UTF_8)), deadlineNanos);
          if (raw == null) {
            throw new SessionEpochFloorMissingException("session epoch floor missing");
          }
          return parsePositive(new String(raw, StandardCharsets.UTF_8));
        });
  }

  @SuppressWarnings("unchecked")
  private static RedisClusterAsyncCommands<byte[], byte[]> commandsFor(RedisConnection connection) {
    Object nativeConnection = connection.getNativeConnection();
    if (!(nativeConnection instanceof RedisClusterAsyncCommands<?, ?>)) {
      throw new SessionEpochFloorUnavailableException("session epoch floor Redis unavailable");
    }
    return (RedisClusterAsyncCommands<byte[], byte[]>) nativeConnection;
  }

  private static <T> T await(Future<T> future, long deadlineNanos) {
    long remainingNanos = deadlineNanos - System.nanoTime();
    if (remainingNanos <= 0) {
      future.cancel(true);
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout");
    }
    try {
      return future.get(remainingNanos, TimeUnit.NANOSECONDS);
    } catch (TimeoutException ex) {
      future.cancel(true);
      throw new SessionEpochFloorUnavailableException("session epoch floor command timeout", ex);
    } catch (InterruptedException ex) {
      future.cancel(true);
      Thread.currentThread().interrupt();
      throw new SessionEpochFloorUnavailableException("session epoch floor command interrupted", ex);
    } catch (ExecutionException ex) {
      Throwable cause = ex.getCause();
      if (cause != null && cause.getMessage() != null && cause.getMessage().contains("invalid session epoch floor")) {
        throw new SessionEpochFloorUnavailableException("invalid session epoch floor", cause);
      }
      throw new SessionEpochFloorUnavailableException("session epoch floor Redis unavailable", cause);
    }
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
