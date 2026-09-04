package voice.backend.auth.sessionepoch;

import java.time.Duration;
import java.util.List;
import org.springframework.dao.DataAccessException;
import org.springframework.data.redis.core.RedisOperations;
import org.springframework.data.redis.core.SessionCallback;
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
            new SessionCallback<>() {
              @Override
              public Long execute(RedisOperations operations) throws DataAccessException {
                while (System.nanoTime() < deadlineNanos && !Thread.currentThread().isInterrupted()) {
                  operations.watch(key);
                  try {
                    String currentRaw = (String) operations.opsForValue().get(key);
                    long current = currentRaw == null ? 0L : parsePositive(currentRaw);
                    long next = Math.max(current, candidate);
                    operations.multi();
                    operations.opsForValue().set(key, Long.toString(next));
                    List<Object> result = operations.exec();
                    if (result != null && !result.isEmpty()) {
                      return next;
                    }
                  } finally {
                    operations.unwatch();
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
    String value = redis.opsForValue().get(key);
    if (value == null) {
      throw new SessionEpochFloorUnavailableException("session epoch floor missing");
    }
    return parsePositive(value);
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
