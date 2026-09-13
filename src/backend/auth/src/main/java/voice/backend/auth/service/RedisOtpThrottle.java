package voice.backend.auth.service;

import java.time.Duration;
import java.util.List;
import org.springframework.dao.DataAccessException;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.script.DefaultRedisScript;
import voice.backend.auth.config.AuthProperties;

/**
 * Auth-owned Redis OTP limits. Each state change is one Lua command so concurrent Auth instances
 * share the same cooldown and verification window. Redis faults deliberately deny the operation.
 */
public class RedisOtpThrottle implements OtpThrottle {
  private static final String RESERVE_SEND_LUA =
      "return redis.call('SET', KEYS[1], '1', 'NX', 'PX', ARGV[1]) and 1 or 0";
  private static final String RECORD_FAILURE_LUA =
      "local count = redis.call('INCR', KEYS[1])\n"
          + "if count == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end\n"
          + "return count";

  private final StringRedisTemplate redis;
  private final String prefix;
  private final long sendCooldownMillis;
  private final long verifyWindowMillis;
  private final int maxVerifyAttempts;
  private final DefaultRedisScript<Long> reserveSendScript = script(RESERVE_SEND_LUA);
  private final DefaultRedisScript<Long> recordFailureScript = script(RECORD_FAILURE_LUA);

  public RedisOtpThrottle(StringRedisTemplate redis, AuthProperties.Redis.Otp properties) {
    if (redis == null || properties == null) {
      throw new IllegalArgumentException("Redis OTP throttle requires Redis and configuration");
    }
    this.redis = redis;
    this.prefix = normalizedPrefix(properties.getPrefix());
    this.sendCooldownMillis = requirePositive(properties.getSendCooldown(), "send cooldown");
    this.verifyWindowMillis = requirePositive(properties.getVerifyWindow(), "verify window");
    this.maxVerifyAttempts = properties.getMaxVerifyAttempts();
    if (maxVerifyAttempts < 1) {
      throw new IllegalArgumentException("max verify attempts must be positive");
    }
  }

  /** Compatibility constructor retaining the documented defaults. */
  public RedisOtpThrottle(StringRedisTemplate redis) {
    this(redis, new AuthProperties.Redis.Otp());
  }

  @Override
  public void reserveSend(String key) {
    Long reserved = execute(reserveSendScript, sendKey(key), Long.toString(sendCooldownMillis));
    if (reserved == null || reserved == 0L) {
      throw new AuthException("otp_rate_limited");
    }
    if (reserved != 1L) {
      unavailable();
    }
  }

  @Override
  public void checkCanVerify(String key) {
    try {
      String value = redis.opsForValue().get(verifyKey(key));
      if (value == null) {
        return;
      }
      long attempts = Long.parseLong(value);
      if (attempts < 0) {
        unavailable();
      }
      if (attempts >= maxVerifyAttempts) {
        throw new AuthException("otp_rate_limited");
      }
    } catch (AuthException ex) {
      throw ex;
    } catch (RuntimeException ex) {
      unavailable(ex);
    }
  }

  @Override
  public void recordFailedVerify(String key) {
    Long attempts = execute(recordFailureScript, verifyKey(key), Long.toString(verifyWindowMillis));
    if (attempts == null || attempts < 1) {
      unavailable();
    }
  }

  private Long execute(DefaultRedisScript<Long> script, String key, String argument) {
    try {
      return redis.execute(script, List.of(key), argument);
    } catch (DataAccessException ex) {
      throw new AuthException("auth_unavailable");
    } catch (RuntimeException ex) {
      throw new AuthException("auth_unavailable");
    }
  }

  private String sendKey(String accountId) {
    return prefix + "send:" + validatedAccountId(accountId);
  }

  private String verifyKey(String accountId) {
    return prefix + "verify:" + validatedAccountId(accountId);
  }

  private static String validatedAccountId(String accountId) {
    if (accountId == null || accountId.isBlank()) {
      throw new AuthException("auth_unavailable");
    }
    return accountId;
  }

  private static DefaultRedisScript<Long> script(String source) {
    DefaultRedisScript<Long> script = new DefaultRedisScript<>();
    script.setScriptText(source);
    script.setResultType(Long.class);
    return script;
  }

  private static long requirePositive(Duration duration, String name) {
    if (duration == null || duration.isZero() || duration.isNegative()) {
      throw new IllegalArgumentException(name + " must be positive");
    }
    return duration.toMillis();
  }

  private static String normalizedPrefix(String value) {
    if (value == null || value.isBlank()) {
      throw new IllegalArgumentException("OTP Redis prefix must not be blank");
    }
    return value.endsWith(":") ? value : value + ":";
  }

  private static void unavailable() {
    throw new AuthException("auth_unavailable");
  }

  private static void unavailable(RuntimeException ignored) {
    throw new AuthException("auth_unavailable");
  }
}
