package voice.backend.auth.service;

import java.time.Duration;
import org.springframework.data.redis.core.StringRedisTemplate;

/** Redis sliding-window OTP throttle. */
public class RedisOtpThrottle implements OtpThrottle {
  private static final Duration SEND_WINDOW = Duration.ofMinutes(1);
  private static final Duration VERIFY_WINDOW = Duration.ofMinutes(10);
  private static final int MAX_VERIFY_ATTEMPTS = 3;

  private final StringRedisTemplate redis;

  public RedisOtpThrottle(StringRedisTemplate redis) {
    this.redis = redis;
  }

  @Override
  public void checkCanSend(String key) {
    if (Boolean.TRUE.equals(redis.hasKey(sendKey(key)))) {
      throw new AuthException("otp_rate_limited");
    }
  }

  @Override
  public void recordSend(String key) {
    redis.opsForValue().set(sendKey(key), "1", SEND_WINDOW);
  }

  @Override
  public void checkCanVerify(String key) {
    String value = redis.opsForValue().get(verifyKey(key));
    if (value != null && Integer.parseInt(value) >= MAX_VERIFY_ATTEMPTS) {
      throw new AuthException("otp_rate_limited");
    }
  }

  @Override
  public void recordFailedVerify(String key) {
    String redisKey = verifyKey(key);
    Long count = redis.opsForValue().increment(redisKey);
    if (count != null && count == 1L) {
      redis.expire(redisKey, VERIFY_WINDOW);
    }
  }

  private static String sendKey(String key) {
    return "otp:send:" + key;
  }

  private static String verifyKey(String key) {
    return "otp:verify:" + key;
  }
}
