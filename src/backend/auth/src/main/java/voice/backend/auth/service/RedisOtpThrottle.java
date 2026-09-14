package voice.backend.auth.service;

import java.time.Duration;
import java.util.List;
import java.util.UUID;
import org.springframework.dao.DataAccessException;
import org.springframework.data.redis.core.StringRedisTemplate;
import org.springframework.data.redis.core.script.DefaultRedisScript;
import voice.backend.auth.config.AuthProperties;

/**
 * Auth-owned Redis OTP limits. Redis Lua serializes every admission across Auth instances. A
 * verification key is a sorted-set sliding window, not a fixed counter window.
 */
public class RedisOtpThrottle implements OtpThrottle {
  private static final String RESERVE_SEND_LUA =
      "if redis.call('SET', KEYS[1], '1', 'NX', 'PX', ARGV[1]) then return 1 end\n"
          + "local marker = redis.call('GET', KEYS[1])\n"
          + "local ttl = redis.call('PTTL', KEYS[1])\n"
          + "if marker == '1' and ttl > 0 then return 0 end\n"
          + "return 2";
  private static final String ADMIT_VERIFY_LUA =
      "local ttl = redis.call('PTTL', KEYS[1])\n"
          + "if ttl == -1 then return redis.error_reply('otp verify state missing ttl') end\n"
          + "local time = redis.call('TIME')\n"
          + "local now = tonumber(time[1]) * 1000 + math.floor(tonumber(time[2]) / 1000)\n"
          + "local window = tonumber(ARGV[1])\n"
          + "local limit = tonumber(ARGV[2])\n"
          + "redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now - window)\n"
          + "local count = redis.call('ZCARD', KEYS[1])\n"
          + "if count == 0 then redis.call('DEL', KEYS[1]) end\n"
          + "if count >= limit then return 0 end\n"
          + "redis.call('ZADD', KEYS[1], now, ARGV[3])\n"
          + "redis.call('PEXPIRE', KEYS[1], window)\n"
          + "return 1";

  private final StringRedisTemplate redis;
  private final String prefix;
  private final long sendCooldownMillis;
  private final long verifyWindowMillis;
  private final int maxVerifyAttempts;
  private final DefaultRedisScript<Long> reserveSendScript = script(RESERVE_SEND_LUA);
  private final DefaultRedisScript<Long> admitVerifyScript = script(ADMIT_VERIFY_LUA);

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
    if (reserved != null && reserved == 0L) {
      throw new AuthException("otp_rate_limited");
    }
    if (reserved == null || reserved != 1L) {
      unavailable();
    }
  }

  @Override
  public void admitVerify(String key) {
    Long admitted =
        execute(
            admitVerifyScript,
            verifyKey(key),
            Long.toString(verifyWindowMillis),
            Integer.toString(maxVerifyAttempts),
            UUID.randomUUID().toString());
    if (admitted == null || admitted == 0L) {
      throw new AuthException("otp_rate_limited");
    }
    if (admitted != 1L) {
      unavailable();
    }
  }

  private Long execute(DefaultRedisScript<Long> script, String key, String... arguments) {
    try {
      return redis.execute(script, List.of(key), (Object[]) arguments);
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
}
