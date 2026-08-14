package voice.backend.auth.service;

import java.time.Duration;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicInteger;

/** In-memory OTP throttle for unit tests (no Redis). */
public class InMemoryOtpThrottle implements OtpThrottle {
  private static final Duration SEND_WINDOW = Duration.ofMinutes(1);
  private static final Duration VERIFY_WINDOW = Duration.ofMinutes(10);
  private static final int MAX_VERIFY_ATTEMPTS = 3;

  private final Map<String, Long> lastSendAt = new ConcurrentHashMap<>();
  private final Map<String, AttemptWindow> verifyAttempts = new ConcurrentHashMap<>();

  @Override
  public void checkCanSend(String key) {
    Long last = lastSendAt.get(key);
    if (last != null && System.currentTimeMillis() - last < SEND_WINDOW.toMillis()) {
      throw new AuthException("otp_rate_limited");
    }
  }

  @Override
  public void recordSend(String key) {
    lastSendAt.put(key, System.currentTimeMillis());
  }

  @Override
  public void checkCanVerify(String key) {
    AttemptWindow window = verifyAttempts.get(key);
    if (window != null && window.isBlocked()) {
      throw new AuthException("otp_rate_limited");
    }
  }

  @Override
  public void recordFailedVerify(String key) {
    verifyAttempts.compute(key, (k, existing) -> {
      AttemptWindow window = existing == null ? new AttemptWindow() : existing;
      window.recordFailure();
      return window;
    });
  }

  private static final class AttemptWindow {
    private long windowStart = System.currentTimeMillis();
    private final AtomicInteger failures = new AtomicInteger();

    boolean isBlocked() {
      if (System.currentTimeMillis() - windowStart > VERIFY_WINDOW.toMillis()) {
        windowStart = System.currentTimeMillis();
        failures.set(0);
        return false;
      }
      return failures.get() >= MAX_VERIFY_ATTEMPTS;
    }

    void recordFailure() {
      if (System.currentTimeMillis() - windowStart > VERIFY_WINDOW.toMillis()) {
        windowStart = System.currentTimeMillis();
        failures.set(0);
      }
      failures.incrementAndGet();
    }
  }
}
